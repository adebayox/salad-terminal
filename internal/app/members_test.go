package app

import "testing"

func TestIsHumanMemberOwnerNotAI(t *testing.T) {
	owner := member{DisplayName: "codex-live-qa", MemberType: "user", Role: "owner", UserID: "abc"}
	if !isHumanMember(owner) {
		t.Fatal("expected owner user to be human")
	}
	if isAIMember(owner) {
		t.Fatal("owner must not count as AI")
	}
	ai := member{DisplayName: "Claude Sonnet", MemberType: "ai", Slug: "claude-sonnet"}
	if isHumanMember(ai) {
		t.Fatal("AI must not count as human")
	}
	if !isAIMember(ai) {
		t.Fatal("expected AI member")
	}
}

func TestMembersFromNamesSkipsHumans(t *testing.T) {
	got := membersFromNames([]string{"codex-live-qa", "Claude Sonnet", "GPT-5.4"}, true)
	if len(got) != 2 {
		t.Fatalf("expected 2 AI names, got %#v", got)
	}
}

func TestRenderMarkdownBold(t *testing.T) {
	out := renderMarkdown("hello **world**", 60)
	if out == "" {
		t.Fatal("empty render")
	}
	if contains := false; true {
		// Rendered output should not keep raw ** markers as the only form.
		if out == "hello **world**" {
			t.Fatalf("markdown was not rendered: %q", out)
		}
		_ = contains
	}
}
