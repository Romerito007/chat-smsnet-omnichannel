package service

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/repository"
	mcpcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/contracts"
	phentity "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
	phrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/repository"
)

// ISPToolBridge implements mcpcontracts.ISPToolBridge: it resolves a
// conversation's assistant (by channel type) to its pinned ISP profile and (a)
// gates which SMSNET servers the AI may use and (b) injects the ISP
// config{type+creds} into a tool call's args server-side. Credentials are loaded
// from the profile here, at the dispatch edge — never offered to the model.
type ISPToolBridge struct {
	assistants repository.AssistantRepository
	profiles   phrepo.ProfileRepository
}

// NewISPToolBridge builds the bridge.
func NewISPToolBridge(assistants repository.AssistantRepository, profiles phrepo.ProfileRepository) *ISPToolBridge {
	return &ISPToolBridge{assistants: assistants, profiles: profiles}
}

// AllowServer gates a SMSNET server for the conversation's channel: no
// assistant/profile → false; read → true; write → only when the profile supports
// liberacao or chamado.
func (b *ISPToolBridge) AllowServer(ctx context.Context, channelID, _ string, write bool) (bool, error) {
	profile, ok, err := b.resolveProfile(ctx, channelID)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if !write {
		return true, nil
	}
	return profileHasWriteAction(profile), nil
}

// Decorate injects the ISP config (and, for writes, an idempotency key) into a
// SMSNET tool call. The client-supplied "config" is always overwritten.
func (b *ISPToolBridge) Decorate(ctx context.Context, in mcpcontracts.DecorateInput) (map[string]any, error) {
	args := in.Args
	if args == nil {
		args = map[string]any{}
	}
	profile, ok, err := b.resolveProfile(ctx, in.ChannelID)
	if err != nil {
		return args, err
	}
	if !ok {
		// No profile: leave args untouched (the tool was gated out of the session;
		// a direct call would fail upstream for missing config).
		return args, nil
	}
	config := map[string]any{
		"type":                            profile.ISPType,
		"usa_pegar_fatura_atrasada":       profile.Options.UsaPegarFaturaAtrasada,
		"usa_extrair_linha_digitavel_pdf": profile.Options.UsaExtrairLinhaDigitavelPDF,
	}
	for k, v := range profile.Credentials {
		config[k] = v
	}
	args["config"] = config
	if in.Write && in.IdempotencyKey != "" {
		args["idempotency_key"] = in.IdempotencyKey
	}
	return args, nil
}

// resolveProfile finds the enabled assistant serving the channel connection id
// and its enabled pinned ISP profile. ok is false (no error) when the channel id
// is empty, there is no assistant, no pinned profile, or the profile is
// missing/disabled. An empty channel id (e.g. a manually-created conversation)
// resolves to no assistant — there is NO fallback by channel type.
func (b *ISPToolBridge) resolveProfile(ctx context.Context, channelID string) (*phentity.ISPProfile, bool, error) {
	if channelID == "" {
		return nil, false, nil
	}
	assistant, err := b.assistants.FindByChannelID(ctx, channelID)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	if assistant.ISPProfileID == "" {
		return nil, false, nil
	}
	profile, err := b.profiles.FindByID(ctx, assistant.ISPProfileID)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	if !profile.Enabled {
		return nil, false, nil
	}
	return profile, true, nil
}

// profileHasWriteAction reports whether the profile's ISP supports a side-effect
// action (liberacao or chamado).
func profileHasWriteAction(p *phentity.ISPProfile) bool {
	for _, a := range p.Actions() {
		if a == phentity.ActionLiberacao || a == phentity.ActionChamado {
			return true
		}
	}
	return false
}

var _ mcpcontracts.ISPToolBridge = (*ISPToolBridge)(nil)
