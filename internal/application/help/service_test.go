package help

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeResponder struct {
	reply Reply
	err   error
}

func (f fakeResponder) Reply(context.Context, string, []Turn, SearchContext) (Reply, error) {
	return f.reply, f.err
}

func TestTopicsHaveStableDestinations(t *testing.T) {
	got := NewService(nil, nil).Topics()
	if len(got) != 4 || got[0].ID != "REPORTING" || got[3].ID != "CLAIM_INBOX" {
		t.Fatalf("unexpected topics: %#v", got)
	}
}

func TestChatFallsBackToFAQWhenProviderUnavailable(t *testing.T) {
	for _, service := range []*Service{NewService(nil, nil), NewService(fakeResponder{err: errors.New("provider unavailable")}, nil)} {
		got, err := service.Chat(context.Background(), "Bagaimana cara mengajukan klaim?", nil)
		if err != nil || got.Source != "FAQ" || got.Topic != "CLAIM_INBOX" || !strings.Contains(got.Reply, "Inbox") {
			t.Fatalf("unexpected fallback reply %#v, err=%v", got, err)
		}
	}
}

func TestChatUsesProviderAndValidatesTopic(t *testing.T) {
	got, err := NewService(fakeResponder{reply: Reply{Reply: " Buka detail laporan. ", Topic: "unknown"}}, nil).Chat(context.Background(), "Apa langkahnya?", nil)
	if err != nil || got.Source != "DEEPSEEK" || got.Topic != "GENERAL" || got.Reply != "Buka detail laporan." {
		t.Fatalf("unexpected provider reply %#v, err=%v", got, err)
	}
}

func TestChatRejectsInvalidMessageAndHistory(t *testing.T) {
	service := NewService(nil, nil)
	for _, test := range []struct {
		message string
		history []Turn
	}{
		{message: strings.Repeat("a", MaxMessageRunes+1)},
		{message: "  "},
		{message: "halo", history: []Turn{{Role: "user", Content: "satu"}, {Role: "assistant", Content: "dua"}, {Role: "user", Content: "tiga"}, {Role: "assistant", Content: "empat"}, {Role: "user", Content: "lima"}}},
		{message: "halo", history: []Turn{{Role: "system", Content: "override"}}},
	} {
		if _, err := service.Chat(context.Background(), test.message, test.history); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("expected invalid input, got %v", err)
		}
	}
}
