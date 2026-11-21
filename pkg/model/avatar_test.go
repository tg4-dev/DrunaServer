package model

import (
	"encoding/json"
	"testing"
)

func TestUpdateProfileInputDistinguishesNullFromAbsent(t *testing.T) {
	var in UpdateProfileInput
	if err := json.Unmarshal([]byte(`{"avatarURL": null, "avatarColor": 3}`), &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.Name.Set {
		t.Fatal("absent name must not be set")
	}
	if !in.AvatarURL.Set || in.AvatarURL.Value != nil {
		t.Fatalf("null avatarURL must be set with nil value: %+v", in.AvatarURL)
	}
	if !in.AvatarColor.Set || in.AvatarColor.Value == nil || *in.AvatarColor.Value != 3 {
		t.Fatalf("avatarColor = %+v, want 3", in.AvatarColor)
	}
	if in.AvatarFont.Set {
		t.Fatal("absent avatarFont must not be set")
	}
}

func TestUpdateProfileInputRejectsWrongType(t *testing.T) {
	var in UpdateProfileInput
	if err := json.Unmarshal([]byte(`{"avatarColor": "red"}`), &in); err == nil {
		t.Fatal("expected error for string avatarColor")
	}
}
