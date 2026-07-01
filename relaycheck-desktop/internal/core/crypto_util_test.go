package core

import (
	"strings"
	"testing"
)

func TestMaskSecret(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "single_char", input: "a", want: "*"},
		{name: "two_chars", input: "ab", want: "**"},
		{name: "three_chars", input: "abc", want: "***"},
		{name: "four_chars", input: "abcd", want: "****"},
		// len("abcde")=5 → max(4, 5-4)=4 stars + "bcde" = "****bcde"
		{name: "five_chars_shows_last4", input: "abcde", want: "****bcde"},
		// len("abcdef")=6 → max(4, 6-4)=4 stars + "cdef" = "****cdef"
		{name: "six_chars_shows_last4", input: "abcdef", want: "****cdef"},
		// len("abcdefg")=7 → max(4, 7-4)=4 stars + "defg" = "****defg"
		{name: "seven_chars_shows_last4", input: "abcdefg", want: "****defg"},
		// len("abcdefgh")=8 → max(4, 8-4)=4 stars + "efgh" = "****efgh"
		{name: "eight_chars_shows_last4", input: "abcdefgh", want: "****efgh"},
		// len("abcdefghi")=9 → max(4, 9-4)=5 stars + "fghi" = "*****fghi"
		{name: "nine_chars_5stars_last4", input: "abcdefghi", want: "*****fghi"},
		// len=19 → max(4, 15)=15 stars + "cdef"
		{name: "typical_api_key", input: "sk-1234567890abcdef", want: "***************cdef"},
		// len(中文密钥测试)=18 UTF-8 bytes → max(4, 14)=14 stars + last 4 bytes (partial last char)
		{name: "unicode_chars", input: "中文密钥测试", want: strings.Repeat("*", 14) + "\x8b\xe8\xaf\x95"},
		{name: "special_chars", input: "p@$$w0rd!", want: "*****0rd!"},
		{name: "whitespace_only_is_not_empty", input: " ", want: "*"},
		{name: "long_secret", input: strings.Repeat("x", 100), want: strings.Repeat("*", 96) + "xxxx"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := maskSecret(tc.input)
			if got != tc.want {
				t.Errorf("maskSecret(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestMaskSecret_PreservesLast4(t *testing.T) {
	// For secrets longer than 4 chars, the last 4 characters must be preserved.
	for _, secret := range []string{"abcde", "sk-longkey123", "p@ss:word/here"} {
		got := maskSecret(secret)
		if len(got) < 4 {
			t.Errorf("maskSecret(%q) result too short: %q", secret, got)
			continue
		}
		suffix := got[len(got)-4:]
		expected := secret[len(secret)-4:]
		if suffix != expected {
			t.Errorf("maskSecret(%q) last 4 = %q, want %q", secret, suffix, expected)
		}
	}
}

func TestMaskSecret_StarsOnlyForShort(t *testing.T) {
	// For secrets of length 1-4, the result should be all asterisks.
	for _, secret := range []string{"a", "ab", "abc", "abcd"} {
		got := maskSecret(secret)
		for i, r := range got {
			if r != '*' {
				t.Errorf("maskSecret(%q)[%d] = %c, want '*'", secret, i, r)
			}
		}
	}
}

func TestSecretFingerprint(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string // prefix check or exact match
		exact bool   // true = exact match, false = prefix only
	}{
		{name: "empty", input: "", want: "", exact: true},
		{name: "whitespace_only", input: "   ", want: "", exact: true},
		{name: "tab_newline_only", input: "\t\n", want: "", exact: true},
		{name: "prefix_key_", input: "sk-abc123", want: "key_", exact: false},
		{name: "trim_before_hash", input: "  sk-abc123  ", want: secretFingerprint("sk-abc123"), exact: true},
		{name: "length_16", input: "any-secret-value", want: "key_", exact: false},
		{name: "deterministic", input: "hello", want: secretFingerprint("hello"), exact: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := secretFingerprint(tc.input)
			if tc.exact {
				if got != tc.want {
					t.Errorf("secretFingerprint(%q) = %q, want %q", tc.input, got, tc.want)
				}
			} else {
				if !strings.HasPrefix(got, tc.want) {
					t.Errorf("secretFingerprint(%q) = %q, want prefix %q", tc.input, got, tc.want)
				}
			}
		})
	}
}

func TestSecretFingerprint_Format(t *testing.T) {
	got := secretFingerprint("test-value")
	if !strings.HasPrefix(got, "key_") {
		t.Errorf("fingerprint should start with 'key_', got %q", got)
	}
	// After "key_" prefix, there should be 12 hex characters = 16 total.
	if len(got) != 16 {
		t.Errorf("fingerprint length = %d, want 16 (key_ + 12 hex chars), got %q", len(got), got)
	}
	hexPart := got[4:]
	for _, c := range hexPart {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("fingerprint hex part contains non-hex char %c in %q", c, got)
			break
		}
	}
}

func TestSecretFingerprint_EmptyIsNotKeyPrefix(t *testing.T) {
	// Empty/whitespace-only input must return empty string, not "key_..."
	got := secretFingerprint("")
	if got != "" {
		t.Errorf("secretFingerprint('') = %q, want empty", got)
	}
	got = secretFingerprint("   \t\n  ")
	if got != "" {
		t.Errorf("secretFingerprint(whitespace) = %q, want empty", got)
	}
}

func TestSecretFingerprint_SameInputSameOutput(t *testing.T) {
	// The same input must always produce the same fingerprint.
	first := secretFingerprint("sk-deterministic-key")
	for i := 0; i < 10; i++ {
		if got := secretFingerprint("sk-deterministic-key"); got != first {
			t.Errorf("secretFingerprint is non-deterministic: got %q on call %d, want %q", got, i+1, first)
		}
	}
}

func TestSecretFingerprint_ShortSecrets(t *testing.T) {
	// Even very short non-empty secrets should produce a valid fingerprint.
	for _, secret := range []string{"x", "a", "1"} {
		got := secretFingerprint(secret)
		if got == "" {
			t.Errorf("secretFingerprint(%q) = empty, want non-empty", secret)
		}
		if !strings.HasPrefix(got, "key_") {
			t.Errorf("secretFingerprint(%q) = %q, want 'key_' prefix", secret, got)
		}
	}
}

func TestSecretFingerprint_UnicodeInput(t *testing.T) {
	// Unicode strings should be fingerprinted without panicking.
	got := secretFingerprint("中文密钥")
	if got == "" {
		t.Error("secretFingerprint with Unicode input returned empty, want non-empty")
	}
	if !strings.HasPrefix(got, "key_") {
		t.Errorf("secretFingerprint(unicode) = %q, want 'key_' prefix", got)
	}
}
