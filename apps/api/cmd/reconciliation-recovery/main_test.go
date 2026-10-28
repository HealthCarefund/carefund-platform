package main

import "testing"

func TestParseAcknowledgeOptionsRequiresExplicitConfirmation(t *testing.T) {
	if _, err := parseAcknowledgeOptions(nil); err == nil {
		t.Fatal("missing confirmation must be rejected")
	}
	if _, err := parseAcknowledgeOptions([]string{"--confirm-gap", "abc", "--resume-ledger", "9000"}); err == nil {
		t.Fatal("confirmation without --yes must be rejected")
	}
	opts, err := parseAcknowledgeOptions([]string{"--confirm-gap", "abc", "--resume-ledger", "9000", "--yes"})
	if err != nil {
		t.Fatalf("valid explicit confirmation rejected: %v", err)
	}
	if opts.confirmedGap != "abc" || opts.resumeLedger != 9000 {
		t.Fatalf("options = %+v", opts)
	}
}

func TestParseResumeLedgerChecksUint32Boundaries(t *testing.T) {
	cases := []struct {
		raw     string
		want    uint32
		wantErr bool
	}{
		{raw: "1", want: 1},
		{raw: "4294967295", want: 4294967295},
		{raw: "0", wantErr: true},
		{raw: "4294967296", wantErr: true},
		{raw: "-1", wantErr: true},
		{raw: "1x", wantErr: true},
		{raw: " 1", wantErr: true},
	}
	for _, test := range cases {
		got, err := parseResumeLedger(test.raw)
		if test.wantErr {
			if err == nil {
				t.Errorf("parseResumeLedger(%q) succeeded with %d, want error", test.raw, got)
			}
			continue
		}
		if err != nil || got != test.want {
			t.Errorf("parseResumeLedger(%q) = %d, %v, want %d", test.raw, got, err, test.want)
		}
	}
}

func TestParseAcknowledgeOptionsRejectsTrailingArguments(t *testing.T) {
	if _, err := parseAcknowledgeOptions([]string{"--confirm-gap", "abc", "--resume-ledger", "1", "--yes", "extra"}); err == nil {
		t.Fatal("trailing arguments must be rejected")
	}
}
