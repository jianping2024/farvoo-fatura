package auth_test

import (
	"testing"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/auth"
)

func TestSignVerifyOperatorToken(t *testing.T) {
	secret := []byte("uat-secret")
	tok, err := auth.SignOperatorToken(secret, auth.OperatorClaims{
		StoreID: "store-1", MesaUserID: "mesa-u1", Role: "cashier",
		TerminalID: "term-1", DisplayName: "Ana",
		Iat: time.Now().Unix(), Exp: time.Now().Add(10 * time.Minute).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err := auth.VerifyOperatorToken(secret, tok)
	if err != nil {
		t.Fatal(err)
	}
	if c.MesaUserID != "mesa-u1" || c.TerminalID != "term-1" {
		t.Fatalf("claims %+v", c)
	}
}

func TestVerifyOperatorTokenRejectsBadSig(t *testing.T) {
	tok, err := auth.SignOperatorToken([]byte("a"), auth.OperatorClaims{MesaUserID: "u"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.VerifyOperatorToken([]byte("b"), tok); err == nil {
		t.Fatal("expected bad signature")
	}
}
