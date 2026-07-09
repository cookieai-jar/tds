package tds

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// interpolateNamed is the driver.NamedValue variant of interpolate. Placeholders are
// positional ("?"), so the argument names/ordinals are used only in slice order.
func interpolateNamed(query string, namedArgs []driver.NamedValue) (string, error) {
	args := make([]driver.Value, len(namedArgs))
	for i := range namedArgs {
		args[i] = namedArgs[i].Value
	}
	return interpolate(query, args)
}

// interpolate replaces positional "?" placeholders in query with the escaped SQL-literal
// form of each arg, so the statement can be sent as a plain language request. Placeholders
// inside single-quoted string literals, double-quoted identifiers, bracketed identifiers
// ([...]), and comments (-- to end of line, and /* ... */) are left untouched. It returns
// an error if the number of placeholders does not match len(args).
//
// This is the opt-in alternative (interpolateParams DSN option) to the ASE dynamic-SQL
// prepared-statement protocol, which hangs on servers that do not implement it such as
// SAP IQ / SQL Anywhere.
func interpolate(query string, args []driver.Value) (string, error) {
	var b strings.Builder
	b.Grow(len(query) + 16*len(args))

	argIdx := 0
	i := 0
	n := len(query)
	for i < n {
		c := query[i]
		switch c {
		case '\'', '"':
			// Copy a quoted region verbatim. A doubled quote ('' or "") is an escaped
			// quote and does not close the region.
			quote := c
			b.WriteByte(c)
			i++
			for i < n {
				b.WriteByte(query[i])
				if query[i] == quote {
					if i+1 < n && query[i+1] == quote {
						b.WriteByte(query[i+1])
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		case '[':
			// Bracketed identifier [ident] (T-SQL / SQL Anywhere compatibility). A doubled
			// ]] is an escaped ] and does not close the identifier.
			b.WriteByte(c)
			i++
			for i < n {
				b.WriteByte(query[i])
				if query[i] == ']' {
					if i+1 < n && query[i+1] == ']' {
						b.WriteByte(query[i+1])
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		case '-':
			// Line comment: -- ... to end of line.
			if i+1 < n && query[i+1] == '-' {
				for i < n && query[i] != '\n' {
					b.WriteByte(query[i])
					i++
				}
			} else {
				b.WriteByte(c)
				i++
			}
		case '/':
			// Block comment: /* ... */.
			if i+1 < n && query[i+1] == '*' {
				b.WriteString("/*")
				i += 2
				for i < n {
					if query[i] == '*' && i+1 < n && query[i+1] == '/' {
						b.WriteString("*/")
						i += 2
						break
					}
					b.WriteByte(query[i])
					i++
				}
			} else {
				b.WriteByte(c)
				i++
			}
		case '?':
			if argIdx >= len(args) {
				return "", fmt.Errorf("tds: more placeholders than args (%d args)", len(args))
			}
			lit, err := encodeLiteral(args[argIdx])
			if err != nil {
				return "", err
			}
			b.WriteString(lit)
			argIdx++
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}

	if argIdx != len(args) {
		return "", fmt.Errorf("tds: parameter count mismatch, %d placeholders but %d args", argIdx, len(args))
	}
	return b.String(), nil
}

// encodeLiteral renders a driver.Value as a SQL literal. database/sql's default converter
// reduces every argument to one of: nil, int64, float64, bool, []byte, string, time.Time.
func encodeLiteral(v driver.Value) (string, error) {
	switch val := v.(type) {
	case nil:
		return "NULL", nil
	case int64:
		return strconv.FormatInt(val, 10), nil
	case float64:
		return strconv.FormatFloat(val, 'g', -1, 64), nil
	case bool:
		if val {
			return "1", nil
		}
		return "0", nil
	case string:
		return quoteStringLiteral(val), nil
	case []byte:
		return hexLiteral(val), nil
	case time.Time:
		// SQL Anywhere accepts the standard 'YYYY-MM-DD HH:MM:SS[.ffffff]' string form.
		return "'" + val.Format("2006-01-02 15:04:05.999999") + "'", nil
	default:
		return "", fmt.Errorf("tds: cannot interpolate argument of type %T", v)
	}
}

// quoteStringLiteral returns a SQL string literal, escaping embedded single quotes by
// doubling them (standard SQL / SQL Anywhere escaping).
func quoteStringLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// hexLiteral renders bytes as a SQL binary literal (0x...).
func hexLiteral(b []byte) string {
	const hexdigits = "0123456789abcdef"
	var sb strings.Builder
	sb.Grow(2 + 2*len(b))
	sb.WriteString("0x")
	for _, c := range b {
		sb.WriteByte(hexdigits[c>>4])
		sb.WriteByte(hexdigits[c&0x0f])
	}
	return sb.String()
}
