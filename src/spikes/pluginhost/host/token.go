package host

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// The capability token follows the JWT in F §3: EdDSA, typ "plgcap+jwt", naming a plugin and what it may
// do. The prototype leaves out that token's tenant and epoch_min claims.
// The host mints it and the host checks it, so the only header ever accepted is the one minted here.
const (
	tokenHeader = `{"alg":"EdDSA","typ":"plgcap+jwt"}`
	tokenAud    = "ascc-mgmt-api"
	tokenTTL    = time.Hour
)

type claims struct {
	Iss  string     `json:"iss"`
	Aud  string     `json:"aud"`
	Sub  string     `json:"sub"`
	Iat  int64      `json:"iat"`
	Exp  int64      `json:"exp"`
	Jti  string     `json:"jti"`
	ASCC asccClaims `json:"ascc"`
}

type asccClaims struct {
	ConnectorID     string   `json:"connector_id"`
	ContractVersion string   `json:"contract_version"`
	Scopes          []string `json:"scopes"`
	NetworkHosts    []string `json:"network_hosts"`
	CredentialKinds []string `json:"credential_kinds"`
}

var b64 = base64.RawURLEncoding

func mint(key ed25519.PrivateKey, c claims) string {
	body, _ := json.Marshal(c) // a struct of strings and ints can't fail to marshal
	signed := b64.EncodeToString([]byte(tokenHeader)) + "." + b64.EncodeToString(body)
	return signed + "." + b64.EncodeToString(ed25519.Sign(key, []byte(signed)))
}

// verify returns the claims only for a token this host signed, with the one header it mints, that
// is for the management API and inside its lifetime. Every other input is an error.
func verify(pub ed25519.PublicKey, token string, now time.Time) (claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims{}, errors.New("not a token")
	}
	header, err := b64.DecodeString(parts[0])
	if err != nil || string(header) != tokenHeader {
		return claims{}, errors.New("wrong token header")
	}
	sig, err := b64.DecodeString(parts[2])
	if err != nil || !ed25519.Verify(pub, []byte(parts[0]+"."+parts[1]), sig) {
		return claims{}, errors.New("bad signature")
	}
	body, err := b64.DecodeString(parts[1])
	if err != nil {
		return claims{}, errors.New("bad claims")
	}
	var c claims
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return claims{}, errors.New("bad claims")
	}
	if c.Aud != tokenAud {
		return claims{}, errors.New("wrong audience")
	}
	if t := now.Unix(); t >= c.Exp || t < c.Iat {
		return claims{}, errors.New("outside its lifetime")
	}
	return c, nil
}
