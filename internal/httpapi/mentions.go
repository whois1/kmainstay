package httpapi

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

type mentionCandidate struct {
	ID   string
	Name string
}

type mentionedUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func mentionedUsers(body string, candidates []mentionCandidate) []mentionedUser {
	bodyRunes := []rune(norm.NFC.String(body))
	seen := map[string]bool{}
	var mentions []mentionedUser
	for index, current := range bodyRunes {
		if current != '@' || (index > 0 && isMentionNameRune(bodyRunes[index-1])) {
			continue
		}
		var longest *mentionCandidate
		for candidateIndex := range candidates {
			candidate := &candidates[candidateIndex]
			nameRunes := []rune(candidate.Name)
			end := index + 1 + len(nameRunes)
			if end > len(bodyRunes) || !strings.EqualFold(string(bodyRunes[index+1:end]), candidate.Name) {
				continue
			}
			if end < len(bodyRunes) && isMentionNameRune(bodyRunes[end]) {
				continue
			}
			if longest == nil || len(nameRunes) > len([]rune(longest.Name)) {
				longest = candidate
			}
		}
		if longest != nil && !seen[longest.ID] {
			mentions = append(mentions, mentionedUser{ID: longest.ID, Name: longest.Name})
			seen[longest.ID] = true
		}
	}
	return mentions
}

func isMentionNameRune(value rune) bool {
	return unicode.IsLetter(value) || unicode.IsDigit(value) || unicode.IsMark(value) || value == '_'
}
