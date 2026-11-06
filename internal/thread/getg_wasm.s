//go:build gc

#include "textflag.h"

// func getg() unsafe.Pointer
TEXT ·getg(SB), NOSPLIT, $0-8
	MOVD g, ret+0(FP)
	RET
