// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-lib.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package collectors

import "testing"

func TestLustreFsname(t *testing.T) {
	tests := []struct {
		llite string
		want  string
	}{
		{"scratch-ffff9a6e4c1bc800", "scratch"},
		{"my-fs", "my-fs"},
		{"a-b-ffff0001", "a-b"},
		{"noseparator", "noseparator"},
		{"-ffff", "-ffff"},
		{"fs-", "fs-"},
	}
	for _, tt := range tests {
		if got := lustreFsname(tt.llite); got != tt.want {
			t.Errorf("lustreFsname(%q) = %q, want %q", tt.llite, got, tt.want)
		}
	}
}
