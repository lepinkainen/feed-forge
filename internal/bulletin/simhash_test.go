package bulletin

import "testing"

func TestSimHashEmpty(t *testing.T) {
	if got := SimHash(""); got != 0 {
		t.Errorf("empty text: got %d, want 0", got)
	}
	if got := SimHash("the a an of"); got != 0 {
		t.Errorf("all-stopword text: got %d, want 0", got)
	}
}

// Near-duplicate article bodies (one edited word) must land within the default
// clustering threshold. This mirrors real use: dedup runs on full text, where
// the same story from two sources shares nearly all tokens.
func TestSimHashSimilarTextIsClose(t *testing.T) {
	a := SimHash("The Federal Communications Commission added foreign made drones to its covered list barring approval and blocking imports of new models it deems a national security risk citing vulnerabilities in data transmission and flight controllers")
	b := SimHash("The Federal Communications Commission added foreign made drones to its covered list barring approval and blocking imports of new models it deems an unacceptable national security risk citing vulnerabilities in data transmission and flight controllers")
	if d := Hamming(a, b); d > defaultSimhashThreshold {
		t.Errorf("near-duplicate distance %d exceeds default threshold %d", d, defaultSimhashThreshold)
	}
}

func TestSimHashDifferentTextIsFar(t *testing.T) {
	a := SimHash("The Federal Communications Commission added foreign made drones to its covered list barring approval and blocking imports of new models it deems a national security risk")
	b := SimHash("Apple unveiled a redesigned MacBook Pro featuring a faster processor a brighter display and substantially improved battery life at its autumn hardware event")
	if d := Hamming(a, b); d < 15 {
		t.Errorf("distinct stories too close: %d", d)
	}
}

// Finnish text (å/ä/ö) must survive tokenization: previously the ASCII-only
// splitter dropped every accented word, collapsing such text to a zero (or
// near-empty) fingerprint. It must now hash to a stable, non-zero value, and two
// distinct Finnish stories must be far apart.
func TestSimHashFinnishText(t *testing.T) {
	s := "Helsingin kaupunginvaltuusto päätti korottaa joukkoliikenteen lippujen hintoja ensi vuoden alusta"
	if SimHash(s) == 0 {
		t.Fatal("Finnish text produced a zero fingerprint (accented tokens were dropped)")
	}
	if d := Hamming(SimHash(s), SimHash(s)); d != 0 {
		t.Errorf("identical Finnish text distance: got %d, want 0", d)
	}
	other := SimHash("Sääennuste lupaa ensi viikolle poikkeuksellisen lämmintä ja aurinkoista säätä koko maahan")
	if d := Hamming(SimHash(s), other); d < 15 {
		t.Errorf("distinct Finnish stories too close: %d", d)
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int // token count
	}{
		{name: "plain english", text: "hello world", want: 2},
		{name: "stopwords dropped", text: "the quick brown fox", want: 3},
		{name: "single chars dropped", text: "a b cd e f", want: 1}, // only "cd"
		{name: "numbers kept", text: "report 2024", want: 2},
		{name: "mixed unicode", text: "Suomi 2024 åäö", want: 3},
		{name: "all stopwords", text: "the a an of in", want: 0},
		{name: "punctuation split", text: "hello, world!", want: 2},
		{name: "empty", text: "", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := len(tokenize(tt.text)); got != tt.want {
				t.Errorf("tokenize(%q) = %d tokens, want %d", tt.text, got, tt.want)
			}
		})
	}
}

func TestSimHashIdentical(t *testing.T) {
	s := "the quick brown fox jumps over the lazy dog repeatedly"
	if d := Hamming(SimHash(s), SimHash(s)); d != 0 {
		t.Errorf("identical text distance: got %d, want 0", d)
	}
}
