package model

import "encoding/json"

// AvatarFonts lists the letter fonts the client can render. Unknown ids are
// rejected so friends never receive a font their client cannot show.
var AvatarFonts = []string{"onest", "playfair", "lobster", "oswald", "unbounded", "comfortaa"}

// AvatarColorCount is the size of the client palette; valid indexes are 0..AvatarColorCount-1.
const AvatarColorCount = 8

// AvatarStyle is how a user's avatar looks to everyone. A nil color or font
// means the user has not chosen one and the client uses its default.
type AvatarStyle struct {
	AvatarURL   string  `json:"avatarURL" db:"avatar_url"`
	AvatarColor *int    `json:"avatarColor" db:"avatar_color"`
	AvatarFont  *string `json:"avatarFont" db:"avatar_font"`
}

func IsAvatarFont(font string) bool {
	for _, f := range AvatarFonts {
		if f == font {
			return true
		}
	}
	return false
}

func IsAvatarColor(color int) bool {
	return color >= 0 && color < AvatarColorCount
}

// OptionalString tells an absent JSON field (Set=false) apart from an
// explicit null (Set=true, Value=nil) and a value.
type OptionalString struct {
	Set   bool
	Value *string
}

func (o *OptionalString) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Value = nil
		return nil
	}
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}

// OptionalInt is OptionalString for integers.
type OptionalInt struct {
	Set   bool
	Value *int
}

func (o *OptionalInt) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Value = nil
		return nil
	}
	var v int
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}
