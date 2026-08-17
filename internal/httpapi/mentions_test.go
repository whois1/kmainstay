package httpapi

import (
	"reflect"
	"testing"
)

func TestMentionedUsers_NaturalNamesAndBoundaries(t *testing.T) {
	candidates := []mentionCandidate{
		{ID: "hector", Name: "Hector"},
		{ID: "mary", Name: "Mary Jane"},
		{ID: "unicode", Name: "Tāne 测试"},
	}
	tests := []struct {
		name string
		body string
		want []mentionedUser
	}{
		{"case insensitive", "please ask @hEcToR!", []mentionedUser{{ID: "hector", Name: "Hector"}}},
		{"spaced name", "hello @Mary Jane, welcome", []mentionedUser{{ID: "mary", Name: "Mary Jane"}}},
		{"unicode name", "kia ora @tĀNE 测试.", []mentionedUser{{ID: "unicode", Name: "Tāne 测试"}}},
		{"email is not a mention", "mail me@example.com", nil},
		{"longer name is not a mention", "hello @Hectorine", nil},
		{"embedded at sign is not a mention", "prefix@Hector", nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := mentionedUsers(test.body, candidates); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("mentionedUsers(%q) = %#v, want %#v", test.body, got, test.want)
			}
		})
	}
}
