package service

import (
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
)

func seedSelf(repo *fakeUserRepo) *entity.User {
	u := &entity.User{ID: "me", TenantID: "t1", Name: "Old", Email: "me@acme.com", PasswordHash: "hashed:current", Status: entity.StatusActive}
	repo.users[u.ID] = u
	return u
}

func strptr(s string) *string { return &s }

func TestUpdateProfile_NameAndAvatar(t *testing.T) {
	svc, repo := newUserService()
	seedSelf(repo)
	ctx := tenantCtx("t1")

	got, err := svc.UpdateProfile(ctx, "me", contracts.UpdateProfile{
		Name: strptr("New Name"), AvatarAttachmentID: strptr("att-1"),
	})
	if err != nil {
		t.Fatalf("update profile: %v", err)
	}
	if got.Name != "New Name" || got.AvatarAttachmentID != "att-1" {
		t.Errorf("profile not updated: %+v", got)
	}
}

func TestUpdateProfile_EmptyNameRejected(t *testing.T) {
	svc, repo := newUserService()
	seedSelf(repo)
	if _, err := svc.UpdateProfile(tenantCtx("t1"), "me", contracts.UpdateProfile{Name: strptr("  ")}); apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation_error, got %v", err)
	}
}

func TestChangePassword_WrongCurrentRejected(t *testing.T) {
	svc, repo := newUserService()
	seedSelf(repo)
	if err := svc.ChangePassword(tenantCtx("t1"), "me", "not-current", "brandnewpass"); apperror.From(err).Code != apperror.CodeUnauthorized {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestChangePassword_TooShortRejected(t *testing.T) {
	svc, repo := newUserService()
	seedSelf(repo)
	if err := svc.ChangePassword(tenantCtx("t1"), "me", "current", "short"); apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation_error, got %v", err)
	}
}

func TestChangePassword_Success(t *testing.T) {
	svc, repo := newUserService()
	seedSelf(repo)
	if err := svc.ChangePassword(tenantCtx("t1"), "me", "current", "brandnewpass"); err != nil {
		t.Fatalf("change password: %v", err)
	}
	if repo.users["me"].PasswordHash != "hashed:brandnewpass" {
		t.Errorf("password not updated, got %q", repo.users["me"].PasswordHash)
	}
}
