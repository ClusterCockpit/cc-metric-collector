// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-lib.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package collectors

import "testing"

func TestInfinibandTypeID(t *testing.T) {
	if got, want := infinibandTypeID("mlx5_0", "1"), "mlx5_0:1"; got != want {
		t.Errorf("infinibandTypeID(%q, %q) = %q, want %q", "mlx5_0", "1", got, want)
	}
}
