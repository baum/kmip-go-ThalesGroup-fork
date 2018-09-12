package kmip

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"
)

var sample = `
420078 | 01 | 00000118 
	420077 | 01 | 00000048 
		420069 | 01 | 00000020 
			42006A | 02 | 00000004 | 0000000100000000
			42006B | 02 | 00000004 | 0000000000000000
		420010 | 06 | 00000008 | 0000000000000001
		42000D | 02 | 00000004 | 0000000200000000
	42000F | 01 | 00000068
		42005C | 05 | 00000004 | 0000000800000000
		420093 | 08 | 00000001 | 3600000000000000
		4200790100000040420008010000003842000A07000000044E616D650000000042000B010000002042005507000000067075626B657900004200540500000004000000010000000042000F010000005042005C05000000040000000E00000000420093080000000137000000000000004200790100000028420008010000002042000A0700000008782D6D796174747242000B07000000057465737432000000`

func TestPrint(t *testing.T) {
	b := hex2bytes(sample)
	t.Log(TTLV(b).String())
}

func TestDecoding(t *testing.T) {
	bi := &big.Int{}
	bi, ok := bi.SetString("1234567890000000000000000000", 10)
	require.True(t, ok)

	dt, err := time.Parse("Monday, January 2, 2006, 15:04:05 MST", "Friday, March 14, 2008, 11:56:40 UTC")
	require.NoError(t, err)

	tests := []struct {
		bs  string
		b   []byte
		exp interface{}
		typ Type
	}{
		{
			bs:  "42 00 20 | 02 | 00 00 00 04 | 00 00 00 08 00 00 00 00",
			exp: int(8),
			typ: TypeInteger,
		},
		{
			bs:  "42 00 20 | 03 | 00 00 00 08 | 01 B6 9B 4B A5 74 92 00",
			exp: int64(123456789000000000),
			typ: TypeLongInteger,
		},
		{
			bs:  "42 00 20 | 04 | 00 00 00 10 | 00 00 00 00 03 FD 35 EB 6B C2 DF 46 18 08 00 00",
			exp: bi,
			typ: TypeBigInteger,
		},
		{
			bs:  "42 00 20 | 05 | 00 00 00 04 | 00 00 00 FF 00 00 00 00",
			exp: EnumInt(255),
			typ: TypeEnumeration,
		},
		{
			bs:  "42 00 20 | 06 | 00 00 00 08 | 00 00 00 00 00 00 00 01",
			exp: true,
			typ: TypeBoolean,
		},
		{
			bs:  "42 00 20 | 07 | 00 00 00 0B | 48 65 6C 6C 6F 20 57 6F 72 6C 64 00 00 00 00 00",
			exp: "Hello World",
			typ: TypeTextString,
		},
		{
			bs:  "42 00 20 | 08 | 00 00 00 03 | 01 02 03 00 00 00 00 00",
			exp: []byte{0x01, 0x02, 0x03},
			typ: TypeByteString,
		},
		{
			bs:  "42 00 20 | 09 | 00 00 00 08 | 00 00 00 00 47 DA 67 F8",
			exp: dt,
			typ: TypeDateTime,
		},
		{
			bs:  "42 00 20 | 0A | 00 00 00 04 | 00 0D 2F 00 00 00 00 00",
			exp: 10 * 24 * time.Hour,
			typ: TypeInterval,
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {

			b := hex2bytes(tc.bs)
			tt := TTLV(b)
			assert.NoError(t, tt.Valid())
			assert.Equal(t, tc.typ, tt.Type())
			assert.Equal(t, tc.exp, tt.Value())
		})
	}

	// structure
	b := hex2bytes("42 00 20 | 01 | 00 00 00 20 | 42 00 04 | 05 | 00 00 00 04 | 00 00 00 FE 00 00 00 00 | 42 00 05 | 02 | 00 00 00 04 | 00 00 00 FF 00 00 00 00")
	tt := TTLV(b)
	assert.NoError(t, tt.Valid())
	assert.Equal(t, TypeStructure, tt.Type())
	exp := hex2bytes("42 00 04 | 05 | 00 00 00 04 | 00 00 00 FE 00 00 00 00 | 42 00 05 | 02 | 00 00 00 04 | 00 00 00 FF 00 00 00 00")
	assert.Equal(t, TTLV(exp), tt.Value())

	for _, test := range knownGoodSamples {
		name := test.name
		if name == "" {
			name = fmt.Sprintf("%T:%v", test.v, test.v)
		}
		t.Run(name, func(t *testing.T) {
			b := hex2bytes(test.exp)
			tt := TTLV(b)
			require.NoError(t, tt.Valid())

			tagBytes := make([]byte, 4)
			copy(tagBytes[1:], b[:3])
			assert.Equal(t, Tag(binary.BigEndian.Uint32(tagBytes)), tt.Tag())

			assert.Equal(t, Type(b[3]), tt.Type())

			assert.Equal(t, int(binary.BigEndian.Uint32(b[4:8])), tt.Len())

			assert.Equal(t, len(b), tt.FullLen())

			// allow permitting type conversions, not exact equality
			// also handle special case of non-pointer big.Ints, which
			// will be decoded as *big.Int.

			switch v := test.v.(type) {
			case big.Int:
				if assert.IsType(t, &v, tt.Value()) {
					assert.True(t, tt.Value().(*big.Int).Cmp(&v) == 0)
				}
			case *big.Int:
				if assert.IsType(t, v, tt.Value()) {
					assert.True(t, tt.Value().(*big.Int).Cmp(v) == 0)
				}
			case TTLV:
				assert.Equal(t, v, tt)
			default:
				assert.EqualValues(t, test.v, tt.Value())
			}

		})
	}
}

func TestTTLV_UnmarshalTTLV(t *testing.T) {
	var ttlv TTLV

	require.Nil(t, ttlv)

	buf := bytes.NewBuffer(nil)
	enc := NewEncoder(buf)
	require.NoError(t, enc.EncodeValue(TagComment, "red"))

	err := ttlv.UnmarshalTTLV(TTLV(buf.Bytes()), false)
	require.NoError(t, err)

	require.NotNil(t, ttlv)
	require.Equal(t, TTLV(buf.Bytes()), ttlv)

	// if ttlv is already allocated and is long enough, allocate
	// into the existing byte slice, rather than allocating a new one
	// (avoid unnecessary allocation for performance)

	ttlv = make(TTLV, buf.Len()+100) // create a TTLV buf a bit larger than necessary
	// copy some marker bytes into the end.  after unmarshaling, the marker bytes should
	// be intact, since they are in the end part of the buffer
	copy(ttlv[buf.Len():], []byte("whitewhale"))
	err = ttlv.UnmarshalTTLV(TTLV(buf.Bytes()), false)

	require.NoError(t, err)
	require.Equal(t, TTLV(buf.Bytes()), ttlv)
	require.Equal(t, buf.Len()+100, cap(ttlv))
	require.Len(t, ttlv, buf.Len())
	require.EqualValues(t, []byte("whitewhale"), ttlv[buf.Len():buf.Len()+10])

	// if ttlv is not nil, but is not long enough to hold TTLV value,
	// everything still works

	ttlv = make(TTLV, buf.Len()-2)
	err = ttlv.UnmarshalTTLV(TTLV(buf.Bytes()), false)

	require.NoError(t, err)
	require.Equal(t, TTLV(buf.Bytes()), ttlv)

}

func TestTTLV_MarshalJSON(t *testing.T) {
	tests := []struct {
		in  interface{}
		exp string
	}{
		{
			in:  TaggedValue{Tag: TagBatchCount, Value: 10},
			exp: `{"tag":"BatchCount","type":"Integer","value":10}`,
		},
		{
			in:  TaggedValue{Tag: Tag(0x540002), Value: 10},
			exp: `{"tag":"0x540002","type":"Integer","value":10}`,
		},
		{
			in:  TaggedValue{Tag: TagBatchCount, Value: `"Red Rover"`},
			exp: `{"tag":"BatchCount","type":"TextString","value":"\"Red Rover\""}`,
		},
		{
			in:  TaggedValue{Tag: TagBatchCount, Value: true},
			exp: `{"tag":"BatchCount","type":"Boolean","value":true}`,
		},
		{
			in:  TaggedValue{Tag: TagBatchCount, Value: false},
			exp: `{"tag":"BatchCount","type":"Boolean","value":false}`,
		},
		{
			in:  TaggedValue{Tag: TagBatchCount, Value: math.MaxInt32},
			exp: `{"tag":"BatchCount","type":"Integer","value":` + strconv.Itoa(math.MaxInt32) + `}`,
		},
		{
			in:  TaggedValue{Tag: TagBatchCount, Value: int64(math.MaxInt32) + 1},
			exp: `{"tag":"BatchCount","type":"LongInteger","value":` + strconv.FormatInt(int64(math.MaxInt32)+1, 10) + `}`,
		},
		{
			// test values higher than max json number, should be encoded in hex
			in: TaggedValue{Tag: TagBatchCount, Value: int64(1) << 53},
			exp: func() string {
				ttlv, err := Marshal(TaggedValue{Tag: TagBatchCount, Value: int64(1) << 53})
				require.NoError(t, err)
				return `{"tag":"BatchCount","type":"LongInteger","value":"0x` + hex.EncodeToString(TTLV(ttlv).ValueRaw()) + `"}`
			}(),
		},
		{
			in:  TaggedValue{Tag: TagBatchCount, Value: big.NewInt(10)},
			exp: `{"tag":"BatchCount","type":"BigInteger","value":10}`,
		},
		{
			// test values higher than max json number, should be encoded in hex
			in: TaggedValue{Tag: TagBatchCount, Value: big.NewInt(int64(1) << 53)},
			exp: func() string {
				ttlv, err := Marshal(TaggedValue{Tag: TagBatchCount, Value: big.NewInt(int64(1) << 53)})
				require.NoError(t, err)
				return `{"tag":"BatchCount","type":"BigInteger","value":"0x` + hex.EncodeToString(TTLV(ttlv).ValueRaw()) + `"}`
			}(),
		},
		{
			in:  TaggedValue{Tag: TagBatchCount, Value: WrappingMethodMACSign},
			exp: `{"tag":"BatchCount","type":"Enumeration","value":"0x00000002"}`,
		},
		{
			in:  TaggedValue{Tag: TagKeyFormatType, Value: KeyFormatTypeX_509},
			exp: `{"tag":"KeyFormatType","type":"Enumeration","value":"X_509"}`,
		},
		{
			in:  TaggedValue{Tag: TagKeyFormatType, Value: EnumInt(0x00050000)},
			exp: `{"tag":"KeyFormatType","type":"Enumeration","value":"0x00050000"}`,
		},
		{
			in: TaggedValue{Tag: TagBatchCount, Value: func() time.Time {
				d, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05+04:00")
				require.NoError(t, err)
				return d
			}()},
			exp: `{"tag":"BatchCount","type":"DateTime","value":"2006-01-02T11:04:05Z"}`,
		},
		{
			in:  TaggedValue{Tag: TagKeyFormatType, Value: 10 * time.Second},
			exp: `{"tag":"KeyFormatType","type":"Interval","value":10}`,
		},
		{
			in: Structure{Tag: TagKeyFormatType, Values: []interface{}{
				TaggedValue{Tag: TagBatchCount, Value: 10},
				TaggedValue{Tag: Tag(0x540002), Value: 10},
				TaggedValue{Tag: TagBatchItem, Value: true},
			}},
			exp: `{"tag":"KeyFormatType","value":[
				{"tag":"BatchCount","type":"Integer","value":10},
				{"tag":"0x540002","type":"Integer","value":10},
				{"tag":"BatchItem","type":"Boolean","value":true}
			]}`,
		},
		{
			in: Structure{Tag: TagAttribute, Values: []interface{}{
				TaggedValue{Tag: TagAttributeName, Value: "Key Format Type"},
				TaggedValue{Tag: TagAttributeValue, Value: KeyFormatTypeX_509},
			}},
			exp: `{"tag":"Attribute","value":[
				{"tag":"AttributeName","type":"TextString","value":"Key Format Type"},
				{"tag":"AttributeValue","type":"Enumeration","value":"X_509"}
			]}`,
		},
		{
			in: Structure{Tag: TagAttribute, Values: []interface{}{
				TaggedValue{Tag: TagAttributeName, Value: "Key Format Type"},
				TaggedValue{Tag: TagAttributeValue, Value: "X_509"},
			}},
			exp: `{"tag":"Attribute","value":[
				{"tag":"AttributeName","type":"TextString","value":"Key Format Type"},
				{"tag":"AttributeValue","type":"TextString","value":"X_509"}
			]}`,
		},
		{
			in: Structure{Tag: TagAttribute, Values: []interface{}{
				TaggedValue{Tag: TagAttributeName, Value: "Key Format Type"},
				TaggedValue{Tag: TagAttributeValue, Value: EnumInt(0x00000300)},
			}},
			exp: `{"tag":"Attribute","value":[
				{"tag":"AttributeName","type":"TextString","value":"Key Format Type"},
				{"tag":"AttributeValue","type":"Enumeration","value":"0x00000300"}
			]}`,
		},
		{
			in:  TaggedValue{Tag: TagCryptographicUsageMask, Value: 10 * time.Second},
			exp: `{"tag":"CryptographicUsageMask","type":"Integer","value":10}`,
		},
	}

	for _, testcase := range tests {
		t.Run("", func(t *testing.T) {
			b, err := Marshal(testcase.in)
			require.NoError(t, err)
			ttlv := TTLV(b)
			j, err := json.Marshal(ttlv)
			require.NoError(t, err)
			require.JSONEq(t, testcase.exp, string(j))
		})
	}
}

// hex2bytes converts hex string to bytes.  Any non-hex characters in the string are stripped first.
// panics on error
func hex2bytes(s string) []byte {
	// strip non hex bytes
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'A' && r <= 'F':
		case r >= 'a' && r <= 'f':
		default:
			return -1 // drop
		}
		return r
	}, s)
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}

	return b
}
