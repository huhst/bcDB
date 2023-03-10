package vrf

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"log"
	"math/big"

	"github.com/r2ishiguro/vrf/go/vrf_ed25519/edwards25519" // copied from "golang.org/x/crypto/ed25519/internal/edwards25519"
	"golang.org/x/crypto/ed25519"
)

const (
	limit    = 100
	N2       = 32 // ceil(log2(q) / 8)
	N        = N2 / 2
	qs       = "1000000000000000000000000000000014def9dea2f79cd65812631a5cf5d3ed" // 2^252 + 27742317777372353535851937790883648493
	cofactor = 8
	NOSING   = 3
)

var (
	ErrMalformedInput = errors.New("ECVRF: malformed input")
	ErrDecodeError    = errors.New("ECVRF: decode error")
	//ErrInternalError  = errors.New("ECVRF: internal error")
	q, _ = new(big.Int).SetString(qs, 16)
	g    = G()
)

const (
	// PublicKeySize is the size, in bytes, of public keys as used in this package.
	PublicKeySize = 32
	// PrivateKeySize is the size, in bytes, of private keys as used in this package.
	PrivateKeySize = 64
	// SignatureSize is the size, in bytes, of signatures generated and verified by this package.
	SignatureSize = 64
)

// EcVrfProve 假设<pk，sk>是由ed25519.GenerateKey（）生成的
func EcVrfProve(pk []byte, sk []byte, m []byte) (pi []byte, err error) {
	x := expandSecret(sk)
	h := EcVrfHashToCurve(m, pk)
	r := ECP2OS(GeScalarMult(h, x))

	kp, ks, err := ed25519.GenerateKey(nil) // use GenerateKey to generate a random
	if err != nil {
		return nil, err
	}
	k := expandSecret(ks)

	// ECVRF_hash_points(g, h, g^x, h^x, g^k, h^k)
	c := EcVrfHashPoints(ECP2OS(g), ECP2OS(h), S2OS(pk), r, S2OS(kp), ECP2OS(GeScalarMult(h, k)))

	// s = k - c*x mod q
	var z big.Int
	s := z.Mod(z.Sub(F2IP(k), z.Mul(c, F2IP(x))), q)

	// pi = gamma || I2OSP(c, N) || I2OSP(s, 2N)
	var buf bytes.Buffer
	buf.Write(r) // 2N
	buf.Write(I2OSP(c, N))
	buf.Write(I2OSP(s, N2))
	return buf.Bytes(), nil
}

func EcVrfProof2hash(pi []byte) []byte {
	return pi[1 : N2+1]
}

func EcVrfVerify(pk []byte, pi []byte, m []byte) (bool, error) {
	r, c, s, err := EcVrfDecodeProof(pi)
	if err != nil {
		return false, err
	}

	// u = (g^x)^c * g^s = P^c * g^s
	var u edwards25519.ProjectiveGroupElement
	P := OS2ECP(pk, pk[31]>>7)
	if P == nil {
		return false, ErrMalformedInput
	}
	edwards25519.GeDoubleScalarMultVartime(&u, c, P, s)

	h := EcVrfHashToCurve(m, pk)

	// v = gamma^c * h^s
	//	fmt.Printf("c, r, s, h\n%s%s%s%s\n", hex.Dump(c[:]), hex.Dump(ECP2OS(r)), hex.Dump(s[:]), hex.Dump(ECP2OS(h)))
	v := GeAdd(GeScalarMult(r, c), GeScalarMult(h, s))

	// c' = ECVRF_hash_points(g, h, g^x, gamma, u, v)
	c2 := EcVrfHashPoints(ECP2OS(g), ECP2OS(h), S2OS(pk), ECP2OS(r), ECP2OSProj(&u), ECP2OS(v))

	return c2.Cmp(F2IP(c)) == 0, nil
}

func EcVrfDecodeProof(pi []byte) (r *edwards25519.ExtendedGroupElement, c *[N2]byte, s *[N2]byte, err error) {
	i := 0
	sign := pi[i]
	i++
	if sign != 2 && sign != 3 {
		return nil, nil, nil, ErrDecodeError
	}
	r = OS2ECP(pi[i:i+N2], sign-2)
	i += N2
	if r == nil {
		return nil, nil, nil, ErrDecodeError
	}

	// swap and expand to make it a field
	c = new([N2]byte)
	for j := N - 1; j >= 0; j-- {
		c[j] = pi[i]
		i++
	}

	// swap to make it a field
	s = new([N2]byte)
	for j := N2 - 1; j >= 0; j-- {
		s[j] = pi[i]
		i++
	}
	return
}

func EcVrfHashPoints(ps ...[]byte) *big.Int {
	h := sha256.New()
	//	fmt.Printf("hash_points:\n")
	for _, p := range ps {
		h.Write(p)
		//		fmt.Printf("%s\n", hex.Dump(p))
	}
	v := h.Sum(nil)
	return OS2IP(v[:N])
}

func EcVrfHashToCurve(m []byte, pk []byte) *edwards25519.ExtendedGroupElement {
	hash := sha256.New()
	for i := int64(0); i < limit; i++ {
		ctr := I2OSP(big.NewInt(i), 4)
		hash.Write(m)
		hash.Write(pk)
		hash.Write(ctr)
		h := hash.Sum(nil)
		hash.Reset()
		if P := OS2ECP(h, NOSING); P != nil {
			// assume cofactor is 2^n
			for j := 1; j < cofactor; j *= 2 {
				P = GeDouble(P)
			}
			return P
		}
	}
	log.Panic("ECVRF_hash_to_curve: couldn't make a point on curve")
	return nil
}

func OS2ECP(os []byte, sign byte) *edwards25519.ExtendedGroupElement {
	P := new(edwards25519.ExtendedGroupElement)
	var buf [32]byte
	copy(buf[:], os)
	if sign == 0 || sign == 1 {
		buf[31] = (sign << 7) | (buf[31] & 0x7f)
	}
	if !P.FromBytes(&buf) {
		return nil
	}
	return P
}

// S2OS 只需在符号八位字节前加上
func S2OS(s []byte) []byte {
	sign := s[31] >> 7     // @@ we should clear the sign bit??
	os := []byte{sign + 2} // Y = 0x02 if positive or 0x03 if negative
	os = append(os, s...)
	return os
}

func ECP2OS(P *edwards25519.ExtendedGroupElement) []byte {
	var s [32]byte
	P.ToBytes(&s)
	return S2OS(s[:])
}

func ECP2OSProj(P *edwards25519.ProjectiveGroupElement) []byte {
	var s [32]byte
	P.ToBytes(&s)
	return S2OS(s[:])
}

func I2OSP(b *big.Int, n int) []byte {
	os := b.Bytes()
	if n > len(os) {
		var buf bytes.Buffer
		buf.Write(make([]byte, n-len(os))) // prepend 0s
		buf.Write(os)
		return buf.Bytes()
	} else {
		return os[:n]
	}
}

func OS2IP(os []byte) *big.Int {
	return new(big.Int).SetBytes(os)
}

// F2IP 将字段号（LittleEndian格式）转换为大整数
func F2IP(f *[32]byte) *big.Int {
	var t [32]byte
	for i := 0; i < 32; i++ {
		t[32-i-1] = f[i]
	}
	return OS2IP(t[:])
}

func IP2F(b *big.Int) *[32]byte {
	os := b.Bytes()
	r := new([32]byte)
	j := len(os) - 1
	for i := 0; i < 32 && j >= 0; i++ {
		r[i] = os[j]
		j--
	}
	return r
}

func G() *edwards25519.ExtendedGroupElement {
	g := new(edwards25519.ExtendedGroupElement)
	var f edwards25519.FieldElement
	edwards25519.FeOne(&f)
	var s [32]byte
	edwards25519.FeToBytes(&s, &f)
	edwards25519.GeScalarMultBase(g, &s) // g = g^1
	return g
}

func expandSecret(sk []byte) *[32]byte {
	// copied from golang.org/x/crypto/ed25519/ed25519.go -- has to be the same
	digest := sha512.Sum512(sk[:32])
	digest[0] &= 248
	digest[31] &= 127
	digest[31] |= 64
	h := new([32]byte)
	copy(h[:], digest[:])
	return h
}

//
// copied from edwards25519.go and const.go in golang.org/x/crypto/ed25519/internal/edwards25519
//
type CachedGroupElement struct {
	yPlusX, yMinusX, Z, T2d edwards25519.FieldElement
}

// d2 is 2*d.
var d2 = edwards25519.FieldElement{
	-21827239, -5839606, -30745221, 13898782, 229458, 15978800, -12551817, -6495438, 29715968, 9444199,
}

func ToCached(r *CachedGroupElement, p *edwards25519.ExtendedGroupElement) {
	edwards25519.FeAdd(&r.yPlusX, &p.Y, &p.X)
	edwards25519.FeSub(&r.yMinusX, &p.Y, &p.X)
	edwards25519.FeCopy(&r.Z, &p.Z)
	edwards25519.FeMul(&r.T2d, &p.T, &d2)
}

func GeAdd(p, qe *edwards25519.ExtendedGroupElement) *edwards25519.ExtendedGroupElement {
	var q CachedGroupElement
	var r edwards25519.CompletedGroupElement
	var t0 edwards25519.FieldElement

	ToCached(&q, qe)

	edwards25519.FeAdd(&r.X, &p.Y, &p.X)
	edwards25519.FeSub(&r.Y, &p.Y, &p.X)
	edwards25519.FeMul(&r.Z, &r.X, &q.yPlusX)
	edwards25519.FeMul(&r.Y, &r.Y, &q.yMinusX)
	edwards25519.FeMul(&r.T, &q.T2d, &p.T)
	edwards25519.FeMul(&r.X, &p.Z, &q.Z)
	edwards25519.FeAdd(&t0, &r.X, &r.X)
	edwards25519.FeSub(&r.X, &r.Z, &r.Y)
	edwards25519.FeAdd(&r.Y, &r.Z, &r.Y)
	edwards25519.FeAdd(&r.Z, &t0, &r.T)
	edwards25519.FeSub(&r.T, &t0, &r.T)

	re := new(edwards25519.ExtendedGroupElement)
	r.ToExtended(re)
	return re
}

func GeDouble(p *edwards25519.ExtendedGroupElement) *edwards25519.ExtendedGroupElement {
	var q edwards25519.ProjectiveGroupElement
	p.ToProjective(&q)
	var rc edwards25519.CompletedGroupElement
	q.Double(&rc)
	r := new(edwards25519.ExtendedGroupElement)
	rc.ToExtended(r)
	return r
}

func ExtendedGroupElementCMove(t, u *edwards25519.ExtendedGroupElement, b int32) {
	edwards25519.FeCMove(&t.X, &u.X, b)
	edwards25519.FeCMove(&t.Y, &u.Y, b)
	edwards25519.FeCMove(&t.Z, &u.Z, b)
	edwards25519.FeCMove(&t.T, &u.T, b)
}

func GeScalarMult(h *edwards25519.ExtendedGroupElement, a *[32]byte) *edwards25519.ExtendedGroupElement {
	q := new(edwards25519.ExtendedGroupElement)
	q.Zero()
	p := h
	for i := uint(0); i < 256; i++ {
		bit := int32(a[i>>3]>>(i&7)) & 1
		t := GeAdd(q, p)
		ExtendedGroupElementCMove(q, t, bit)
		p = GeDouble(p)
	}
	return q
}
