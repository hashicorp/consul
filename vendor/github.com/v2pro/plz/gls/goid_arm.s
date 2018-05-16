// Copyright 2016 Huan Du. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

#include "go_asm.h"
#include "textflag.h"

TEXT ·getg(SB), NOSPLIT, $0-4
    MOVW    g, R8
    MOVW    R8, ret+0(FP)
    RET