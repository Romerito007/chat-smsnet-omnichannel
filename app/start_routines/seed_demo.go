package start_routines

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"

	auditentity "github.com/romerito007/chat-smsnet-omnichannel/domain/audit/entity"
	bhentity "github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/entity"
	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	channelservice "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/service"
	contactentity "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	contactservice "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/service"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	ctentity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversationtools/entity"
	cpentity "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
	csatentity "github.com/romerito007/chat-smsnet-omnichannel/domain/csat/entity"
	iamentity "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	mcpentity "github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/entity"
	privacyentity "github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/entity"
	phentity "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
	queueentity "github.com/romerito007/chat-smsnet-omnichannel/domain/queues/entity"
	sectorentity "github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/entity"
	slaentity "github.com/romerito007/chat-smsnet-omnichannel/domain/sla/entity"
	whentity "github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/entity"

	auditrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/audit"
	bhrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/businesshours"
	channelrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/channels"
	contactrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/contacts"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversations"
	ctrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversationtools"
	cprepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/copilot"
	csatrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/csat"
	iamrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/iam"
	mcprepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/mcp"
	privacyrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/privacy"
	phrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/providerhub"
	queuerepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/queues"
	sectorrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/sectors"
	slarepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/sla"
	whrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/webhooks"
)

// demoSeedLedger is the collection that records every document the demo seed
// creates ({tenant_id, coll, doc_id}). It is the isolation marker (a "metadado
// equivalente" to a demo_seed:true field): the reset deletes ONLY the ids it
// recorded, in the target tenant, so untagged production docs are never touched.
const demoSeedLedger = "demo_seed_records"

// SeedDemoData seeds a rich, realistic demo dataset into the owner's tenant.
// It is gated by SEED_DEMO_DATA (default false) and is idempotent: if demo data
// already exists it is skipped, unless SEED_DEMO_RESET is set (then only the
// previously demo-seeded docs are wiped and recreated, refreshing relative dates).
// It targets exclusively the tenant of SEED_OWNER_EMAIL and never scans others.
func SeedDemoData(ctx context.Context, c *container.Container) error {
	if !c.Config.Seed.DemoData {
		return nil
	}

	db := c.Mongo.DB
	owner, err := iamrepo.NewUserRepository(db).FindByEmailAnyTenant(ctx, c.Config.Seed.OwnerEmail)
	if err != nil {
		c.Logger.Warn("demo seed skipped: owner user not found", "email", c.Config.Seed.OwnerEmail)
		return nil
	}
	tenantID := owner.TenantID
	tctx := shared.WithTenant(ctx, tenantID)

	d := &demoSeeder{
		ctx: tctx, db: db, c: c, tenantID: tenantID, ownerID: owner.ID,
		now: time.Now().UTC(), rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	seeded, err := d.alreadySeeded()
	if err != nil {
		return fmt.Errorf("demo seed: check ledger: %w", err)
	}
	if seeded {
		if !c.Config.Seed.DemoReset {
			c.Logger.Info("demo já semeado, pulando", "tenant_id", tenantID)
			return nil
		}
		c.Logger.Info("demo reset: apagando dados demo e recriando", "tenant_id", tenantID)
		if err := d.reset(); err != nil {
			return fmt.Errorf("demo seed: reset: %w", err)
		}
	}

	if err := d.run(); err != nil {
		return fmt.Errorf("demo seed: %w", err)
	}
	c.Logger.Info("demo data seeded",
		"tenant_id", tenantID, "users", len(d.agentIDs)+1, "contacts", len(d.contactIDs),
		"conversations", d.convCount)
	return nil
}

// ── seeder state ──────────────────────────────────────────────────────────────

type demoSeeder struct {
	ctx      context.Context
	db       *mongo.Database
	c        *container.Container
	tenantID string
	ownerID  string
	now      time.Time
	rng      *rand.Rand

	// resolved during run
	sectorIDs       map[string]string   // name -> id
	queueIDs        map[string][]string // sector name -> queue ids
	agentIDs        []string            // demo agent user ids
	agentBySec      map[string][]string // sector name -> agent ids
	tagIDs          []string            // canonical tag ids (conversations/contacts store ids, never names)
	channels        []string            // channel type slugs created
	contactIDs      []string
	closeReasn      []string // close reason names
	surveyID        string
	convCount       int
	ownerAssignLeft int // open conversations still to assign to the owner ("Minhas")
}

// mark records a created doc in the ledger immediately (best-effort) so a partial
// run is still fully resettable via SEED_DEMO_RESET.
func (d *demoSeeder) mark(coll, id string) {
	if _, err := d.db.Collection(demoSeedLedger).InsertOne(d.ctx,
		bson.M{"tenant_id": d.tenantID, "coll": coll, "doc_id": id}); err != nil {
		d.c.Logger.Warn("demo seed: ledger insert failed", "coll", coll, "error", err)
	}
}

func (d *demoSeeder) alreadySeeded() (bool, error) {
	n, err := d.db.Collection(demoSeedLedger).CountDocuments(d.ctx, bson.M{"tenant_id": d.tenantID})
	return n > 0, err
}

func (d *demoSeeder) reset() error {
	cur, err := d.db.Collection(demoSeedLedger).Find(d.ctx, bson.M{"tenant_id": d.tenantID})
	if err != nil {
		return err
	}
	byColl := map[string][]string{}
	for cur.Next(d.ctx) {
		var rec struct {
			Coll  string `bson:"coll"`
			DocID string `bson:"doc_id"`
		}
		if err := cur.Decode(&rec); err != nil {
			_ = cur.Close(d.ctx)
			return err
		}
		byColl[rec.Coll] = append(byColl[rec.Coll], rec.DocID)
	}
	_ = cur.Close(d.ctx)
	if err := cur.Err(); err != nil {
		return err
	}
	for coll, ids := range byColl {
		if _, err := d.db.Collection(coll).DeleteMany(d.ctx,
			bson.M{"_id": bson.M{"$in": ids}, "tenant_id": d.tenantID}); err != nil {
			return err
		}
	}
	// Belt-and-suspenders: a partial earlier run may have left duplicate tag slugs
	// that are not in the ledger. Dedupe tags by slug — but ONLY within the target
	// tenant, never across tenants.
	if err := d.dedupeTags(); err != nil {
		return err
	}
	_, err = d.db.Collection(demoSeedLedger).DeleteMany(d.ctx, bson.M{"tenant_id": d.tenantID})
	return err
}

// tagIDByName returns the id of the tenant tag with this slug, or "" if none.
func (d *demoSeeder) tagIDByName(name string) (string, error) {
	var doc struct {
		ID string `bson:"_id"`
	}
	err := d.db.Collection("tags").FindOne(d.ctx,
		bson.M{"tenant_id": d.tenantID, "name": name}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "", nil
		}
		return "", err
	}
	return doc.ID, nil
}

// dedupeTags removes duplicate tag slugs in the target tenant, keeping one per
// slug. Scoped strictly to d.tenantID.
func (d *demoSeeder) dedupeTags() error {
	cur, err := d.db.Collection("tags").Find(d.ctx, bson.M{"tenant_id": d.tenantID})
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	var dupIDs []string
	for cur.Next(d.ctx) {
		var t struct {
			ID   string `bson:"_id"`
			Name string `bson:"name"`
		}
		if err := cur.Decode(&t); err != nil {
			_ = cur.Close(d.ctx)
			return err
		}
		if seen[t.Name] {
			dupIDs = append(dupIDs, t.ID)
		} else {
			seen[t.Name] = true
		}
	}
	_ = cur.Close(d.ctx)
	if err := cur.Err(); err != nil {
		return err
	}
	if len(dupIDs) == 0 {
		return nil
	}
	_, err = d.db.Collection("tags").DeleteMany(d.ctx,
		bson.M{"_id": bson.M{"$in": dupIDs}, "tenant_id": d.tenantID})
	return err
}

// run executes every seed block (4.1–4.11). Optional blocks (4.12/4.13) are
// guarded and skipped — external Customer360 payloads are never persisted, and
// presence lives in Redis (ephemeral).
func (d *demoSeeder) run() error {
	steps := []struct {
		name string
		fn   func() error
	}{
		// Sectors MUST be seeded before the team so agents get real sector ids
		// (d.sectorIDs is populated here); otherwise the lookup returns "" and
		// agents end up with sector_ids: [""], which breaks sector assignment.
		{"sectors_queues", d.seedSectorsQueues},
		{"team", d.seedTeam},
		{"business_hours", d.seedBusinessHours},
		{"sla", d.seedSLAPolicy},
		{"privacy", d.seedPrivacy},
		{"taxonomy_csat", d.seedTaxonomyCSAT},
		{"integrations", d.seedIntegrations},
		{"contacts", d.seedContacts},
		{"conversations", d.seedConversations},
		{"copilot_suggestions", d.seedCopilotApprovals},
		{"audit", d.seedAudit},
	}
	for _, s := range steps {
		if err := s.fn(); err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
	}
	return nil
}

// ── 4.2 sectors & queues (seeded before team so agents get sector_ids) ─────────

func (d *demoSeeder) seedSectorsQueues() error {
	repo := sectorrepo.New(d.db)
	qrepo := queuerepo.New(d.db)
	d.sectorIDs = map[string]string{}
	d.queueIDs = map[string][]string{}

	sectors := []string{"Suporte Técnico", "Financeiro", "Comercial", "Retenção"}
	for _, name := range sectors {
		sec := newSector(d.tenantID, name, d.now)
		if err := repo.Create(d.ctx, sec); err != nil {
			return err
		}
		d.mark("sectors", sec.ID)
		d.sectorIDs[name] = sec.ID
	}

	queues := []struct{ sector, name string }{
		{"Suporte Técnico", "Suporte N1"},
		{"Suporte Técnico", "Suporte N2"},
		{"Financeiro", "Cobrança"},
		{"Comercial", "Vendas"},
		{"Retenção", "Retenção"},
	}
	for _, q := range queues {
		queue := newQueue(d.tenantID, d.sectorIDs[q.sector], q.name, d.now)
		if err := qrepo.Create(d.ctx, queue); err != nil {
			return err
		}
		d.mark("queues", queue.ID)
		d.queueIDs[q.sector] = append(d.queueIDs[q.sector], queue.ID)
	}
	return nil
}

// ── 4.1 team (admin + 4 agents) ────────────────────────────────────────────────

func (d *demoSeeder) seedTeam() error {
	repo := iamrepo.NewUserRepository(d.db)
	hash, err := d.c.Hasher.Hash(d.c.Config.Seed.DemoPassword)
	if err != nil {
		return err
	}
	adminRole, err := d.roleID(authz.DefaultRoleAdmin)
	if err != nil {
		return fmt.Errorf("resolve admin role: %w", err)
	}
	agentRole, err := d.roleID(authz.DefaultRoleAgent)
	if err != nil {
		return fmt.Errorf("resolve agent role: %w", err)
	}
	d.agentBySec = map[string][]string{}

	// Admin (sectors are seeded already).
	admin := newUser(d.tenantID, "Ana Admin", "admin@demo.local", hash, []string{adminRole}, nil, 0, d.now)
	if err := repo.Create(d.ctx, admin); err != nil {
		return err
	}
	d.mark("users", admin.ID)

	agents := []struct{ name, email, sector string }{
		{"Bruno Suporte", "bruno@demo.local", "Suporte Técnico"},
		{"Carla Suporte", "carla@demo.local", "Suporte Técnico"},
		{"Diego Vendas", "diego@demo.local", "Comercial"},
		{"Erica Financeiro", "erica@demo.local", "Financeiro"},
	}
	for _, a := range agents {
		secID := d.sectorIDs[a.sector]
		if secID == "" {
			return fmt.Errorf("sector %q not seeded before team; cannot assign agent %q", a.sector, a.email)
		}
		u := newUser(d.tenantID, a.name, a.email, hash, []string{agentRole}, []string{secID}, 5, d.now)
		if err := repo.Create(d.ctx, u); err != nil {
			return err
		}
		d.mark("users", u.ID)
		d.agentIDs = append(d.agentIDs, u.ID)
		d.agentBySec[a.sector] = append(d.agentBySec[a.sector], u.ID)
	}
	return nil
}

func (d *demoSeeder) roleID(name string) (string, error) {
	var doc struct {
		ID string `bson:"_id"`
	}
	err := d.db.Collection("roles").FindOne(d.ctx, bson.M{"tenant_id": d.tenantID, "name": name}).Decode(&doc)
	return doc.ID, err
}

// ── 4.3 business hours ──────────────────────────────────────────────────────────

func (d *demoSeeder) seedBusinessHours() error {
	// Schedule is stored on the Sector's business_hours map. Apply the same
	// Mon–Fri 08–18 / Sat 08–12 / Sun closed (America/Sao_Paulo) to every sector.
	hours := map[string]any{
		"timezone": "America/Sao_Paulo",
		"weekly": map[string]any{
			"monday":    []any{map[string]any{"start": "08:00", "end": "18:00"}},
			"tuesday":   []any{map[string]any{"start": "08:00", "end": "18:00"}},
			"wednesday": []any{map[string]any{"start": "08:00", "end": "18:00"}},
			"thursday":  []any{map[string]any{"start": "08:00", "end": "18:00"}},
			"friday":    []any{map[string]any{"start": "08:00", "end": "18:00"}},
			"saturday":  []any{map[string]any{"start": "08:00", "end": "12:00"}},
		},
	}
	for _, id := range d.sectorIDs {
		if _, err := d.db.Collection("sectors").UpdateOne(d.ctx,
			bson.M{"_id": id, "tenant_id": d.tenantID},
			bson.M{"$set": bson.M{"business_hours": hours, "updated_at": d.now}}); err != nil {
			return err
		}
	}
	// One demo holiday.
	h := &bhentity.Holiday{
		ID: shared.NewID(), TenantID: d.tenantID, Date: d.now.AddDate(0, 1, 0).Format("2006-01-02"),
		Name: "Feriado Demo", Scope: bhentity.ScopeAllSectors, Recurring: false,
		CreatedAt: d.now, UpdatedAt: d.now,
	}
	if err := bhrepo.NewHolidayRepository(d.db).Create(d.ctx, h); err != nil {
		return err
	}
	d.mark("holidays", h.ID)
	return nil
}

// ── 4.4 SLA policy ──────────────────────────────────────────────────────────────

func (d *demoSeeder) seedSLAPolicy() error {
	repo := slarepo.NewPolicyRepository(d.db)
	for name, secID := range d.sectorIDs {
		p := &slaentity.SLAPolicy{
			ID: shared.NewID(), TenantID: d.tenantID, Name: "SLA " + name,
			SectorIDs: []string{secID}, FirstResponseTargetSec: 15 * 60, ResolutionTargetSec: 24 * 3600,
			WarningThresholdPct: 80, PauseOnWaitingCustomer: true, Enabled: true,
			CreatedAt: d.now, UpdatedAt: d.now,
		}
		if err := repo.Create(d.ctx, p); err != nil {
			return err
		}
		d.mark("sla_policies", p.ID)
	}
	return nil
}

// ── 4.5 privacy / retention ────────────────────────────────────────────────────

func (d *demoSeeder) seedPrivacy() error {
	p := &privacyentity.RetentionPolicy{
		TenantID: d.tenantID, MessagesDays: 365, ClosedConversationsDays: 180,
		TechnicalLogsDays: 30, AuditLogsDays: 365, NotificationsDays: 60, UpdatedAt: d.now,
	}
	if err := privacyrepo.New(d.db).SaveRetention(d.ctx, p); err != nil {
		return err
	}
	d.mark("retention_policies", d.tenantID)
	return nil
}

// ── 4.6 taxonomy & CSAT ────────────────────────────────────────────────────────

func (d *demoSeeder) seedTaxonomyCSAT() error {
	trepo := ctrepo.NewTagRepository(d.db)
	tags := []struct{ name, color string }{
		{"vip", "#F59E0B"}, {"inadimplente", "#EF4444"}, {"sem-conexao", "#3B82F6"},
		{"lentidao", "#8B5CF6"}, {"mudanca-endereco", "#10B981"}, {"cancelamento", "#6B7280"},
		{"segunda-via", "#14B8A6"}, {"financeiro", "#22C55E"},
	}
	for _, t := range tags {
		// Idempotent: reuse the existing tag's id when the slug already exists
		// (never create a duplicate); conversations/contacts store the ID.
		id, err := d.tagIDByName(t.name)
		if err != nil {
			return err
		}
		if id == "" {
			tag := &ctentity.Tag{
				ID: shared.NewID(), TenantID: d.tenantID, Name: t.name, Color: t.color,
				Enabled: true, CreatedAt: d.now, UpdatedAt: d.now,
			}
			if err := trepo.Create(d.ctx, tag); err != nil {
				return err
			}
			d.mark("tags", tag.ID)
			id = tag.ID
		}
		d.tagIDs = append(d.tagIDs, id)
	}

	crepo := ctrepo.NewCannedResponseRepository(d.db)
	canned := []struct{ shortcut, title, body string }{
		{"/oi", "Saudação", "Olá! Sou da equipe de atendimento. Como posso ajudar?"},
		{"/cpf", "Pedido de CPF", "Para localizar seu cadastro, pode me informar o CPF do titular?"},
		{"/verificando", "Verificando", "Aguarde um instante, estou verificando isso para você."},
		{"/encerrar", "Encerramento", "Posso ajudar em algo mais? Caso contrário, vou encerrar por aqui. Obrigado!"},
		{"/boleto", "Envio de boleto", "Segue a segunda via do seu boleto. Qualquer dúvida, estou à disposição."},
		{"/visita", "Agendamento de visita", "Posso agendar uma visita técnica. Qual o melhor período para você?"},
	}
	for _, cr := range canned {
		c := &ctentity.CannedResponse{
			ID: shared.NewID(), TenantID: d.tenantID, Shortcut: cr.shortcut, Title: cr.title,
			Body: cr.body, Enabled: true, CreatedAt: d.now, UpdatedAt: d.now,
		}
		if err := crepo.Create(d.ctx, c); err != nil {
			return err
		}
		d.mark("canned_responses", c.ID)
	}

	rrepo := ctrepo.NewCloseReasonRepository(d.db)
	reasons := []string{"Resolvido", "Sem retorno do cliente", "Duplicado", "Transferido p/ campo", "Cancelamento efetivado"}
	for _, name := range reasons {
		r := &ctentity.CloseReason{
			ID: shared.NewID(), TenantID: d.tenantID, Name: name, Enabled: true,
			CreatedAt: d.now, UpdatedAt: d.now,
		}
		if err := rrepo.Create(d.ctx, r); err != nil {
			return err
		}
		d.mark("close_reasons", r.ID)
		d.closeReasn = append(d.closeReasn, name)
	}

	survey := &csatentity.CSATSurvey{
		ID: shared.NewID(), TenantID: d.tenantID, Name: "Pesquisa de satisfação",
		Scale: csatentity.ScaleCSAT15, QuestionText: "Como você avalia nosso atendimento?",
		SendOn: csatentity.SendOnConversationClosed, Enabled: true, CreatedAt: d.now, UpdatedAt: d.now,
	}
	if err := csatrepo.NewSurveyRepository(d.db).Create(d.ctx, survey); err != nil {
		return err
	}
	d.mark("csat_surveys", survey.ID)
	d.surveyID = survey.ID
	return nil
}

// ── 4.7 integrations ────────────────────────────────────────────────────────────

func (d *demoSeeder) seedIntegrations() error {
	// ProviderHub: a real ISP slug, placeholder base_url + encrypted api key.
	// Skip if the tenant already has one (never clobber a real config).
	phRepo := phrepo.NewConfigRepository(d.db, d.c.Cipher)
	if _, err := phRepo.FindEnabled(d.ctx); err != nil {
		ph := &phentity.ProviderIntegrationConfig{
			ID: shared.NewID(), TenantID: d.tenantID, Name: "SMSNet Demo",
			SMSNetBaseURL: "https://smsnet.demo.local", SMSNetAPIKey: "demo-smsnet-key-PLACEHOLDER",
			ISPType: phentity.ISPSGP, Enabled: true, TimeoutMs: 8000, CreatedAt: d.now, UpdatedAt: d.now,
		}
		if err := phRepo.Create(d.ctx, ph); err != nil {
			return err
		}
		d.mark("providerhub_configs", ph.ID)
	}

	// MCP: one example server.
	srv := &mcpentity.ServerConnection{
		ID: shared.NewID(), TenantID: d.tenantID, Name: "smsnet-integrations",
		Transport: mcpentity.TransportStreamableHTTP, BaseURL: "https://mcp.demo.local/smsnet",
		AuthHeader: "Authorization", AuthToken: "demo-mcp-token-PLACEHOLDER", Kind: mcpentity.KindRead,
		Enabled: true, CreatedAt: d.now, UpdatedAt: d.now,
	}
	if err := mcprepo.NewServerRepository(d.db, d.c.Cipher).Create(d.ctx, srv); err != nil {
		return err
	}
	d.mark("mcp_servers", srv.ID)

	// Channels: WhatsApp, API and a Custom "E-mail" (no native email type). The
	// ConnectionService generates the inbound token (stored hashed); its plaintext
	// is logged once at INFO so dev can inject inbound calls.
	connSvc := channelservice.NewConnectionService(
		channelrepo.NewConnectionRepository(d.db, d.c.Cipher), nil, shared.SystemClock{})
	suporteSec := d.sectorIDs["Suporte Técnico"]
	chans := []struct {
		name string
		typ  chentity.Type
	}{
		{"WhatsApp Oficial", chentity.TypeWhatsApp},
		{"API Genérica", chentity.TypeAPI},
		{"E-mail", chentity.TypeCustom},
	}
	for _, ch := range chans {
		conn, err := connSvc.Create(d.ctx, chcontracts.CreateConnection{
			Type: ch.typ, Name: ch.name, BaseURL: "https://gateway.demo.local/" + string(ch.typ),
			AuthType: chentity.AuthToken, DefaultSectorID: suporteSec, AutomationEnabled: false,
		})
		if err != nil {
			return err
		}
		d.mark("channel_connections", conn.ID)
		d.channels = append(d.channels, string(ch.typ))
		d.c.Logger.Info("demo channel inbound token (dev only)",
			"channel", conn.Name, "type", string(conn.Type), "inbound_token", conn.InboundToken)
	}

	// Webhooks: one example subscription.
	enabled := true
	wh := &whentity.WebhookSubscription{
		ID: shared.NewID(), TenantID: d.tenantID, Name: "Demo webhook",
		URL: "https://hooks.demo.local/chat", Events: []string{"conversation.closed"},
		Secret: "demo-webhook-secret-PLACEHOLDER", Enabled: enabled, RateLimitPerMin: 120,
		CreatedBy: d.ownerID, CreatedAt: d.now, UpdatedAt: d.now,
	}
	if err := whrepo.NewSubscriptionRepository(d.db, d.c.Cipher).Create(d.ctx, wh); err != nil {
		return err
	}
	d.mark("webhook_subscriptions", wh.ID)

	// Copilot: openai/gpt-4o-mini, encrypted key, gates (external on, financial off,
	// human approval on). system_prompt/propose-write are not config fields.
	cpRepo := cprepo.NewConfigRepository(d.db, d.c.Cipher)
	if _, err := cpRepo.FindByTenant(d.ctx); err != nil {
		// No placeholder API key: a fake key would make the copilot look configured
		// but fail every call with a provider auth error. Left empty so has_api_key
		// is false (the UI prompts to configure) and the env-default key, if set,
		// is used as the fallback.
		cfg := &cpentity.AIConfig{
			ID: shared.NewID(), TenantID: d.tenantID, Provider: cpentity.ProviderOpenAI,
			Model: "gpt-4o-mini", APIKey: "", Temperature: 0.3, MaxTokens: 1024,
			AllowCustomerData: true, AllowFinancialData: false, AllowMonitoringData: false,
			HumanApprovalRequired: true, Enabled: true, CreatedAt: d.now, UpdatedAt: d.now,
		}
		if err := cpRepo.Create(d.ctx, cfg); err != nil {
			return err
		}
		d.mark("copilot_configs", cfg.ID)
	}
	return nil
}

// ── 4.8 contacts ────────────────────────────────────────────────────────────────

func (d *demoSeeder) seedContacts() error {
	repo := contactrepo.New(d.db)
	first := []string{"Ana", "Bruno", "Carla", "Diego", "Erica", "Fábio", "Gabriela", "Henrique",
		"Isabela", "João", "Karina", "Lucas", "Marina", "Nelson", "Olívia", "Paulo", "Queila",
		"Rafael", "Sabrina", "Tiago", "Ursula", "Vitor", "Wesley", "Yara", "Zélia"}
	last := []string{"Silva", "Souza", "Oliveira", "Santos", "Pereira", "Lima", "Costa", "Almeida",
		"Ferreira", "Rodrigues", "Gomes", "Martins", "Araújo", "Barbosa", "Ribeiro"}

	// add is the single write path: it normalizes phone (E.164) and document
	// (CPF/CNPJ) with the SAME domain functions the create/update validation uses,
	// so every seeded contact passes that validation (editing it never 400s).
	add := func(name, phone, doc, email string, tags []string) error {
		e164, ok := contactservice.NormalizePhoneE164(phone)
		if !ok {
			return fmt.Errorf("seed: invalid phone %q for contact %q", phone, name)
		}
		document, ok := contactservice.NormalizeDocument(doc)
		if !ok {
			return fmt.Errorf("seed: invalid document %q for contact %q", doc, name)
		}
		ct := &contactentity.Contact{
			ID: shared.NewID(), TenantID: d.tenantID, Name: name, Phone: e164,
			Phones: []string{e164}, Document: document, Email: email, Tags: tags,
			CreatedAt: d.now, UpdatedAt: d.now,
		}
		if err := repo.Create(d.ctx, ct); err != nil {
			return err
		}
		d.mark("contacts", ct.ID)
		d.contactIDs = append(d.contactIDs, ct.ID)
		return nil
	}

	// The existing real contact. Tags store the canonical tag ID (never the name),
	// consistent with the conversation/contact tag-id normalization.
	vipID, err := d.tagIDByName("vip")
	if err != nil {
		return err
	}
	var vipTags []string
	if vipID != "" {
		vipTags = []string{vipID}
	}
	if err := add("Romerito Alexandre", "+5544999088478", "", "", vipTags); err != nil {
		return err
	}
	for i := 0; i < 34; i++ {
		name := first[d.rng.Intn(len(first))] + " " + last[d.rng.Intn(len(last))]
		phone := d.validE164Phone()
		doc, email := "", ""
		if d.rng.Intn(2) == 0 {
			doc = contactservice.GenerateValidCPF(d.rng)
		}
		if d.rng.Intn(2) == 0 {
			email = fmt.Sprintf("cliente%d@demo.local", i+1)
		}
		var tags []string
		if d.rng.Intn(3) == 0 {
			tags = []string{d.tagIDs[d.rng.Intn(len(d.tagIDs))]}
		}
		if err := add(name, phone, doc, email, tags); err != nil {
			return err
		}
	}
	return nil
}

// validE164Phone generates a Brazilian mobile and returns it in E.164, looping
// until libphonenumber accepts it (via the same NormalizePhoneE164 the backend
// validation uses) — so the seeded number is exactly what an edit would accept.
// Uses real area codes (DDDs) and the 9-prefixed 9-digit mobile format.
func (d *demoSeeder) validE164Phone() string {
	ddds := []string{"11", "19", "21", "27", "31", "41", "44", "47", "48", "51", "61", "62", "71", "81", "85"}
	for {
		ddd := ddds[d.rng.Intn(len(ddds))]
		raw := fmt.Sprintf("+55%s9%08d", ddd, d.rng.Intn(100000000))
		if e164, ok := contactservice.NormalizePhoneE164(raw); ok {
			return e164
		}
	}
}

// ── 4.9 conversations + messages + timeline + sla + csat ───────────────────────

func (d *demoSeeder) seedConversations() error {
	// Seed ~4 of the open assigned conversations onto the owner so the owner's
	// "Minhas" tab is not empty when reviewing the layout. These stay open
	// (StatusAssigned, never resolved/closed) and remain reassignable.
	d.ownerAssignLeft = 4
	// Distribution: ~10 open assigned, ~6 queued, ~6 pending, ~4 at-risk, ~24 resolved.
	specs := []struct {
		count    int
		status   conventity.Status
		assign   bool
		atRisk   bool
		resolved bool
	}{
		{10, conventity.StatusAssigned, true, false, false},
		{6, conventity.StatusQueued, false, false, false},
		{6, conventity.StatusWaitingCustomer, true, false, false},
		{4, conventity.StatusAssigned, true, true, false},
		{24, conventity.StatusResolved, true, false, true},
	}
	automationLeft := 1 // attach one automation event to the first conversation
	for _, sp := range specs {
		for i := 0; i < sp.count; i++ {
			if err := d.createConversation(sp.status, sp.assign, sp.atRisk, sp.resolved, &automationLeft); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *demoSeeder) createConversation(status conventity.Status, assign, atRisk, resolved bool, automationLeft *int) error {
	sectorNames := []string{"Suporte Técnico", "Financeiro", "Comercial", "Retenção"}
	sectorName := sectorNames[d.rng.Intn(len(sectorNames))]
	sectorID := d.sectorIDs[sectorName]
	queueID := ""
	if qs := d.queueIDs[sectorName]; len(qs) > 0 {
		queueID = qs[d.rng.Intn(len(qs))]
	}
	contactID := d.contactIDs[d.rng.Intn(len(d.contactIDs))]
	channel := d.channels[d.rng.Intn(len(d.channels))]

	assignee := ""
	if assign {
		switch {
		case !resolved && !atRisk && d.ownerAssignLeft > 0:
			// First few open, non-at-risk assigned conversations go to the owner.
			assignee = d.ownerID
			d.ownerAssignLeft--
		default:
			if a := d.agentBySec[sectorName]; len(a) > 0 {
				assignee = a[d.rng.Intn(len(a))]
			} else {
				assignee = d.agentIDs[d.rng.Intn(len(d.agentIDs))]
			}
		}
	}

	// Timestamps: resolved spread over 14 days; live ones recent (minutes/hours).
	var createdAt time.Time
	if resolved {
		createdAt = d.now.Add(-time.Duration(d.rng.Intn(14*24)) * time.Hour)
	} else if atRisk {
		createdAt = d.now.Add(-time.Duration(20+d.rng.Intn(120)) * time.Minute)
	} else {
		createdAt = d.now.Add(-time.Duration(d.rng.Intn(7*24)) * time.Hour)
	}
	lastMsgAt := createdAt.Add(time.Duration(5+d.rng.Intn(180)) * time.Minute)
	if lastMsgAt.After(d.now) {
		lastMsgAt = d.now
	}

	priorities := []conventity.Priority{conventity.PriorityLow, conventity.PriorityNormal, conventity.PriorityHigh}
	priority := priorities[d.rng.Intn(len(priorities))]
	if atRisk {
		priority = conventity.PriorityHigh
	}

	tags := []string{d.tagIDs[d.rng.Intn(len(d.tagIDs))]}
	if d.rng.Intn(2) == 0 {
		tags = append(tags, d.tagIDs[d.rng.Intn(len(d.tagIDs))])
	}

	conv := &conventity.Conversation{
		ID: shared.NewID(), TenantID: d.tenantID, ContactID: contactID, Channel: channel,
		SectorID: sectorID, QueueID: queueID, Status: status, AssignedTo: assignee,
		Priority: priority, Tags: tags, LastMessageAt: lastMsgAt,
		CreatedAt: createdAt, UpdatedAt: lastMsgAt,
	}
	if !resolved && status != conventity.StatusQueued && d.rng.Intn(2) == 0 {
		conv.UnreadCount = 1 + d.rng.Intn(4)
		lr := createdAt.Add(time.Minute)
		conv.LastReadAt = &lr
	}
	if resolved {
		closed := lastMsgAt.Add(time.Duration(1+d.rng.Intn(30)) * time.Minute)
		conv.ClosedAt = &closed
	}
	if err := convrepo.NewConversationRepository(d.db).Create(d.ctx, conv); err != nil {
		return err
	}
	d.mark("conversations", conv.ID)
	d.convCount++

	if err := d.createMessages(conv, assignee, resolved); err != nil {
		return err
	}
	if err := d.createEvents(conv, assignee, resolved, automationLeft); err != nil {
		return err
	}
	if err := d.createTracking(conv, sectorID, atRisk, resolved, assign); err != nil {
		return err
	}
	if resolved && d.rng.Intn(10) < 7 {
		if err := d.createCSAT(conv, assignee); err != nil {
			return err
		}
	}
	return nil
}

var demoTopics = []string{
	"minha internet caiu agora",
	"internet muito lenta à noite",
	"preciso da segunda via do boleto",
	"quero mudar o endereço de instalação",
	"recebi uma cobrança indevida",
	"gostaria de fazer upgrade de plano",
	"o Wi-Fi não alcança o quarto dos fundos",
	"quero agendar uma visita técnica",
	"pedido de acordo / liberação de confiança",
	"quero cancelar meu plano",
}

func (d *demoSeeder) createMessages(conv *conventity.Conversation, assignee string, resolved bool) error {
	repo := convrepo.NewMessageRepository(d.db)
	topic := demoTopics[d.rng.Intn(len(demoTopics))]
	n := 4 + d.rng.Intn(9)
	t := conv.CreatedAt
	step := conv.LastMessageAt.Sub(conv.CreatedAt) / time.Duration(n+1)
	if step <= 0 {
		step = time.Minute
	}
	agentReplies := []string{
		"Olá! Já estou verificando aqui para você.",
		"Entendi. Pode confirmar o CPF do titular, por favor?",
		"Identifiquei o problema, vou resolver agora.",
		"Acabei de reiniciar o sinal do seu equipamento, pode testar?",
		"Agendei o atendimento. Mais alguma coisa?",
	}
	for i := 0; i < n; i++ {
		t = t.Add(step)
		fromCustomer := i%2 == 0
		m := &conventity.Message{
			ID: shared.NewID(), TenantID: d.tenantID, ConversationID: conv.ID, CreatedAt: t,
			MessageType: conventity.MessageText,
		}
		if fromCustomer {
			m.SenderType = conventity.SenderCustomer
			m.SenderID = conv.ContactID
			m.Direction = conventity.DirectionInbound
			if i == 0 {
				m.Text = topic
			} else {
				m.Text = "ok, obrigado pelo retorno"
			}
		} else {
			m.SenderType = conventity.SenderAgent
			m.SenderID = assignee
			m.Direction = conventity.DirectionOutbound
			m.DeliveryStatus = conventity.DeliveryDelivered
			m.Text = agentReplies[d.rng.Intn(len(agentReplies))]
		}
		if err := repo.Create(d.ctx, m); err != nil {
			return err
		}
		d.mark("messages", m.ID)
	}
	// An internal note on some conversations (never delivered to the customer).
	// Pick from a small pool so a contact's history doesn't show the same note
	// text twice (an internal note is per-conversation, so identical text across
	// two conversations of the same contact looks like a duplicate).
	if assignee != "" && d.rng.Intn(3) == 0 {
		notes := []string{
			"Nota interna: cliente recorrente, tratar com prioridade.",
			"Nota interna: já houve contato anterior sobre o mesmo assunto.",
			"Nota interna: cliente prefere atendimento por WhatsApp.",
			"Nota interna: aguardando retorno do financeiro antes de prosseguir.",
			"Nota interna: verificar histórico de pagamentos antes de oferecer plano.",
		}
		note := &conventity.Message{
			ID: shared.NewID(), TenantID: d.tenantID, ConversationID: conv.ID,
			SenderType: conventity.SenderAgent, SenderID: assignee, Direction: conventity.DirectionInternal,
			MessageType: conventity.MessageText, Text: notes[d.rng.Intn(len(notes))],
			CreatedAt: conv.LastMessageAt,
		}
		if err := repo.Create(d.ctx, note); err != nil {
			return err
		}
		d.mark("messages", note.ID)
	}
	return nil
}

func (d *demoSeeder) createEvents(conv *conventity.Conversation, assignee string, resolved bool, automationLeft *int) error {
	repo := convrepo.NewEventRepository(d.db)
	add := func(typ string, at time.Time, actorType conventity.ActorType, actorID string, data map[string]any) error {
		e := &conventity.ConversationEvent{
			ID: shared.NewID(), TenantID: d.tenantID, ConversationID: conv.ID, Type: typ,
			ActorType: actorType, ActorID: actorID, Data: data, CreatedAt: at,
		}
		if err := repo.Create(d.ctx, e); err != nil {
			return err
		}
		d.mark("conversation_events", e.ID)
		return nil
	}
	if err := add(conventity.EventConversationCreated, conv.CreatedAt, conventity.ActorSystem, "system", nil); err != nil {
		return err
	}
	if automationLeft != nil && *automationLeft > 0 {
		*automationLeft--
		if err := add(conventity.EventAutomationDecision, conv.CreatedAt.Add(time.Second*5),
			conventity.ActorAutomation, "automation", map[string]any{"decision": "route_to_human"}); err != nil {
			return err
		}
	}
	if assignee != "" {
		if err := add(conventity.EventConversationAssigned, conv.CreatedAt.Add(time.Minute),
			conventity.ActorSystem, "system", map[string]any{"assigned_to": assignee}); err != nil {
			return err
		}
	} else {
		if err := add(conventity.EventConversationEnqueued, conv.CreatedAt.Add(time.Minute),
			conventity.ActorSystem, "system", map[string]any{"queue_id": conv.QueueID}); err != nil {
			return err
		}
	}
	if len(conv.Tags) > 0 {
		if err := add(conventity.EventConversationTagged, conv.CreatedAt.Add(2*time.Minute),
			conventity.ActorAgent, assignee, map[string]any{"tags": conv.Tags}); err != nil {
			return err
		}
	}
	if resolved && conv.ClosedAt != nil {
		reason := d.closeReasn[d.rng.Intn(len(d.closeReasn))]
		if err := add(conventity.EventConversationClosed, *conv.ClosedAt,
			conventity.ActorAgent, assignee, map[string]any{"reason": reason}); err != nil {
			return err
		}
	}
	return nil
}

func (d *demoSeeder) createTracking(conv *conventity.Conversation, sectorID string, atRisk, resolved, assign bool) error {
	repo := slarepo.NewTrackingRepository(d.db)
	t := &slaentity.SLATracking{
		ID: shared.NewID(), TenantID: d.tenantID, ConversationID: conv.ID, SectorID: sectorID,
		CreatedAt: conv.CreatedAt, UpdatedAt: d.now,
	}
	switch {
	case atRisk:
		due := d.now.Add(time.Duration(10+d.rng.Intn(30)) * time.Minute)
		warn := d.now.Add(-5 * time.Minute)
		t.Status = slaentity.StatusRunning
		t.ResolutionDueAt = &due
		t.ResolutionWarnAt = &warn
		t.ResolutionWarned = true
	case resolved:
		fr := conv.CreatedAt.Add(10 * time.Minute)
		t.Status = slaentity.StatusMet
		t.FirstResponseAt = &fr
		t.ResolvedAt = conv.ClosedAt
	case assign:
		due := d.now.Add(time.Duration(2+d.rng.Intn(20)) * time.Hour)
		t.Status = slaentity.StatusRunning
		t.ResolutionDueAt = &due
	default:
		// queued: no tracking yet
		return nil
	}
	if err := repo.Create(d.ctx, t); err != nil {
		return err
	}
	d.mark("sla_trackings", t.ID)
	return nil
}

func (d *demoSeeder) createCSAT(conv *conventity.Conversation, assignee string) error {
	score := 1 + d.rng.Intn(5)
	resp := &csatentity.CSATResponse{
		ID: shared.NewID(), TenantID: d.tenantID, ConversationID: conv.ID, ContactID: conv.ContactID,
		SurveyID: d.surveyID, AgentID: assignee, Token: shared.NewID(), Score: &score,
		Status: csatentity.StatusResponded, SentAt: conv.ClosedAt, RespondedAt: conv.ClosedAt,
		CreatedAt: conv.CreatedAt, UpdatedAt: d.now,
	}
	if err := csatrepo.NewResponseRepository(d.db).Create(d.ctx, resp); err != nil {
		return err
	}
	d.mark("csat_responses", resp.ID)
	return nil
}

// ── 4.10 copilot — a pending write-action approval ─────────────────────────────

func (d *demoSeeder) seedCopilotApprovals() error {
	// Persisted artifact of the write-action flow: a pending approval that respects
	// human_approval_required. (Generated suggestions are ephemeral and not stored.)
	var conv struct {
		ID string `bson:"_id"`
	}
	err := d.db.Collection("conversations").FindOne(d.ctx,
		bson.M{"tenant_id": d.tenantID, "status": string(conventity.StatusAssigned)}).Decode(&conv)
	if err != nil {
		return nil // no assigned conversation — skip silently
	}
	ap := &mcpentity.Approval{
		ID: shared.NewID(), TenantID: d.tenantID, ConversationID: conv.ID,
		ServerName: "smsnet-integrations", Tool: "liberar_acesso",
		Args:   map[string]any{"id_cliente": "DEMO-123", "motivo": "liberação de confiança"},
		Status: mcpentity.ApprovalPending, ProposedBy: "copilot", CreatedAt: d.now,
	}
	if err := mcprepo.NewApprovalRepository(d.db).Create(d.ctx, ap); err != nil {
		return err
	}
	d.mark("mcp_approvals", ap.ID)
	return nil
}

// ── 4.11 audit ─────────────────────────────────────────────────────────────────

func (d *demoSeeder) seedAudit() error {
	repo := auditrepo.New(d.db)
	actions := []struct {
		action, resource string
	}{
		{"auth.login", "user"}, {"conversation.assigned", "conversation"},
		{"conversation.transferred", "conversation"}, {"conversation.closed", "conversation"},
		{"message.sent", "message"}, {"conversation.tagged", "conversation"},
		{"providerhub.liberacao", "conversation"},
	}
	actors := append([]string{d.ownerID}, d.agentIDs...)
	for i := 0; i < 30; i++ {
		a := actions[d.rng.Intn(len(actions))]
		at := d.now.Add(-time.Duration(d.rng.Intn(14*24)) * time.Hour)
		log := &auditentity.AuditLog{
			ID: shared.NewID(), TenantID: d.tenantID, ActorID: actors[d.rng.Intn(len(actors))],
			ActorType: shared.ActorTypeUser, Action: a.action, ResourceType: a.resource,
			ResourceID: shared.NewID(), IP: fmt.Sprintf("177.0.0.%d", 1+d.rng.Intn(254)),
			UserAgent: "DemoSeed/1.0", Data: map[string]any{"demo": true}, CreatedAt: at,
		}
		if err := repo.Create(d.ctx, log); err != nil {
			return err
		}
		d.mark("audit_logs", log.ID)
	}
	return nil
}

// ── entity constructors (kept tiny so the seed reads top-down) ──────────────────

func newSector(tenantID, name string, now time.Time) *sectorentity.Sector {
	return &sectorentity.Sector{
		ID: shared.NewID(), TenantID: tenantID, Name: name, Enabled: true,
		CreatedAt: now, UpdatedAt: now,
	}
}

func newQueue(tenantID, sectorID, name string, now time.Time) *queueentity.Queue {
	return &queueentity.Queue{
		ID: shared.NewID(), TenantID: tenantID, SectorID: sectorID, Name: name,
		Strategy: queueentity.StrategyRoundRobin, MaxWaitSeconds: 300, Enabled: true,
		CreatedAt: now, UpdatedAt: now,
	}
}

func newUser(tenantID, name, email, hash string, roleIDs, sectorIDs []string, maxChats int, now time.Time) *iamentity.User {
	return &iamentity.User{
		ID: shared.NewID(), TenantID: tenantID, Name: name, Email: email, PasswordHash: hash,
		Status: iamentity.StatusActive, RoleIDs: roleIDs, SectorIDs: sectorIDs,
		MaxConcurrentChats: maxChats, CreatedAt: now, UpdatedAt: now,
	}
}
