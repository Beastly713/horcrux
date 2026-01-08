package shamir

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"
)

// polynomial represents a polynomial of arbitrary degree
type polynomial struct {
	coefficients []uint8
}

// makePolynomial constructs a random polynomial of the given
// degree but with the provided intercept value.
func makePolynomial(intercept, degree uint8) (polynomial, error) {
	p := polynomial{
		coefficients: make([]byte, degree+1),
	}

	p.coefficients[0] = intercept

	if _, err := rand.Read(p.coefficients[1:]); err != nil {
		return p, err
	}

	return p, nil
}

// evaluate returns the value of the polynomial for the given x
func (p *polynomial) evaluate(x uint8) uint8 {
	if x == 0 {
		return p.coefficients[0]
	}

	degree := len(p.coefficients) - 1
	out := p.coefficients[degree]
	for i := degree - 1; i >= 0; i-- {
		coeff := p.coefficients[i]
		out = add(mult(out, x), coeff)
	}
	return out
}

// interpolatePolynomial takes N sample points and returns
// the value at a given x using a lagrange interpolation.
func interpolatePolynomial(x_samples, y_samples []uint8, x uint8) uint8 {
	limit := len(x_samples)
	var result, basis uint8
	for i := 0; i < limit; i++ {
		basis = 1
		for j := 0; j < limit; j++ {
			if i == j {
				continue
			}
			num := add(x, x_samples[j])
			denom := add(x_samples[i], x_samples[j])
			term := div(num, denom)
			basis = mult(basis, term)
		}
		group := mult(y_samples[i], basis)
		result = add(result, group)
	}
	return result
}

// div divides two numbers in GF(2^8)
func div(a, b uint8) uint8 {
	if b == 0 {
		panic("divide by zero")
	}

	log_a := logTable[a]
	log_b := logTable[b]
	diff := (int(log_a) - int(log_b)) % 255
	if diff < 0 {
		diff += 255
	}

	ret := expTable[diff]

	if subtle.ConstantTimeByteEq(a, 0) == 1 {
		ret = 0
	}

	return ret
}

// mult multiplies two numbers in GF(2^8)
func mult(a, b uint8) (out uint8) {
	log_a := logTable[a]
	log_b := logTable[b]
	sum := (int(log_a) + int(log_b)) % 255

	ret := expTable[sum]

	if subtle.ConstantTimeByteEq(a, 0) == 1 {
		ret = 0
	}

	if subtle.ConstantTimeByteEq(b, 0) == 1 {
		ret = 0
	}

	return ret
}

// add combines two numbers in GF(2^8). Symmetric with subtraction.
func add(a, b uint8) uint8 {
	return a ^ b
}

// Split divides a secret into `parts` shares, requiring `threshold` to reconstruct.
func Split(secret []byte, parts, threshold int) ([][]byte, error) {
	if parts < threshold {
		return nil, fmt.Errorf("parts cannot be less than threshold")
	}
	if parts > 255 {
		return nil, fmt.Errorf("parts cannot exceed 255")
	}
	if threshold < 2 {
		return nil, fmt.Errorf("threshold must be at least 2")
	}
	if len(secret) == 0 {
		return nil, fmt.Errorf("cannot split empty secret")
	}

	// Output format: [y1, y2... yN, x]
	// We use the last byte to store the x-coordinate.
	out := make([][]byte, parts)
	for idx := range out {
		out[idx] = make([]byte, len(secret)+1)
		out[idx][len(secret)] = uint8(idx) + 1 // Assign X coords 1..N
	}

	for idx, val := range secret {
		p, err := makePolynomial(val, uint8(threshold-1))
		if err != nil {
			return nil, err
		}

		for i := 0; i < parts; i++ {
			x := out[i][len(secret)]
			y := p.evaluate(x)
			out[i][idx] = y
		}
	}

	return out, nil
}

// Combine reconstructs the secret from the provided parts.
func Combine(parts [][]byte) ([]byte, error) {
	if len(parts) < 2 {
		return nil, fmt.Errorf("less than two parts cannot reconstruct secret")
	}

	firstLen := len(parts[0])
	secret := make([]byte, firstLen-1)

	xSamples := make([]uint8, len(parts))
	ySamples := make([]uint8, len(parts))

	// Collect X coordinates from the last byte of each part
	for i, part := range parts {
		if len(part) != firstLen {
			return nil, fmt.Errorf("parts length mismatch")
		}
		xSamples[i] = part[firstLen-1]
	}

	// Interpolate for each byte index
	for idx := range secret {
		for i, part := range parts {
			ySamples[i] = part[idx]
		}
		val := interpolatePolynomial(xSamples, ySamples, 0)
		secret[idx] = val
	}

	return secret, nil
}