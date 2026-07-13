package serde

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	gojson "github.com/goccy/go-json"
	jsoniter "github.com/json-iterator/go"
)

var jsoniterCompat = jsoniter.ConfigCompatibleWithStandardLibrary

var benchmarkJSONSink []byte

// benchSizes cover four orders of magnitude of payload size.
var benchSizes = []struct {
	name  string
	users int
}{
	{"small", 1},      // ~0.4 KB
	{"medium", 100},   // ~40 KB
	{"large", 1000},   // ~400 KB
	{"xlarge", 10000}, // ~4 MB
}

func benchFeed(users int) Feed {
	feed := Feed{}
	for i := range users {
		feed.Users = append(feed.Users, sampleUserBench(i))
	}
	return feed
}

func benchFeedPayload(users int) []byte {
	data, err := json.Marshal(toStdBench(benchFeed(users)))
	if err != nil {
		panic(err)
	}
	return data
}

func sampleUserBench(i int) User {
	return User{
		ID:     int64(i) * 7919,
		Name:   fmt.Sprintf("User %d with a plain ascii name", i),
		Email:  fmt.Sprintf("user%d@example.com", i),
		Age:    20 + i%60,
		Active: i%2 == 0,
		Score:  float64(i) * 1.25,
		Tags:   []string{"alpha", "beta", "gamma"},
		Address: Address{
			Street: "1234 Long Street Name Ave",
			City:   "Barcelona",
			Zip:    "08001",
		},
		Items: []Item{
			{SKU: "sku-1", Qty: 2, Price: 9.99},
			{SKU: "sku-2", Qty: 5, Price: 19.5},
			{SKU: "sku-3", Qty: 1, Price: 0.5},
		},
		Metadata: map[string]string{"source": "bench", "region": "eu"},
	}
}

func toStdBench(f Feed) StdFeed {
	data, err := json.Marshal(f)
	if err != nil {
		panic(err)
	}
	var s StdFeed
	if err := json.Unmarshal(data, &s); err != nil {
		panic(err)
	}
	return s
}

func BenchmarkUnmarshal(b *testing.B) {
	for _, size := range benchSizes {
		payload := benchFeedPayload(size.users)
		codecs := []struct {
			name string
			fn   func([]byte) error
		}{
			{"serde", func(d []byte) error {
				var f Feed
				return f.UnmarshalJSON(d)
			}},
			{"stdlib", func(d []byte) error {
				var f StdFeed
				return json.Unmarshal(d, &f)
			}},
			{"easyjson", func(d []byte) error {
				var f EasyFeed
				return f.UnmarshalJSON(d)
			}},
			{"goccy", func(d []byte) error {
				var f StdFeed
				return gojson.Unmarshal(d, &f)
			}},
			{"jsoniter", func(d []byte) error {
				var f StdFeed
				return jsoniterCompat.Unmarshal(d, &f)
			}},
			{"sonic", func(d []byte) error {
				var f StdFeed
				return sonic.Unmarshal(d, &f)
			}},
		}
		for _, c := range codecs {
			b.Run(size.name+"/"+c.name, func(b *testing.B) {
				b.SetBytes(int64(len(payload)))
				b.ReportAllocs()
				for b.Loop() {
					if err := c.fn(payload); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func BenchmarkMarshal(b *testing.B) {
	for _, size := range benchSizes {
		feed := benchFeed(size.users)
		stdFeed := toStdBench(feed)
		var easyFeed EasyFeed
		if err := easyFeed.UnmarshalJSON(benchFeedPayload(size.users)); err != nil {
			b.Fatal(err)
		}
		payloadLen := int64(len(benchFeedPayload(size.users)))
		codecs := []struct {
			name string
			fn   func() ([]byte, error)
		}{
			{"serde", func() ([]byte, error) { return feed.MarshalJSON() }},
			{"stdlib", func() ([]byte, error) { return json.Marshal(stdFeed) }},
			{"easyjson", func() ([]byte, error) { return easyFeed.MarshalJSON() }},
			{"goccy", func() ([]byte, error) { return gojson.Marshal(stdFeed) }},
			{"jsoniter", func() ([]byte, error) { return jsoniterCompat.Marshal(stdFeed) }},
			{"sonic", func() ([]byte, error) { return sonic.Marshal(stdFeed) }},
		}
		for _, c := range codecs {
			b.Run(size.name+"/"+c.name, func(b *testing.B) {
				b.SetBytes(payloadLen)
				b.ReportAllocs()
				for b.Loop() {
					if _, err := c.fn(); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func BenchmarkCompatibilityShapes(b *testing.B) {
	integer := NamedInt(math.MinInt64)
	integerPointer := &integer
	shapes := []struct {
		name      string
		value     CompatibilityValues
		marshal   func() ([]byte, error)
		unmarshal func([]byte) error
	}{
		{
			name:  "escaped_strings",
			value: CompatibilityValues{String: strings.Repeat("<&>\u2028日本語\\\"\n", 256), NamedStringSlice: []NamedString{"", "plain", "日本語"}},
		},
		{
			name: "scalar_containers",
			value: CompatibilityValues{
				Slice: []int{math.MinInt, -1, 0, 1, math.MaxInt}, Int8Slice: []int8{math.MinInt8, 0, math.MaxInt8},
				Float32Slice: []float32{math.SmallestNonzeroFloat32, -0, math.MaxFloat32},
				Map:          map[string]int{"negative": -1, "zero": 0, "positive": 1}, Bytes: make([]byte, 64<<10),
			},
		},
		{
			name: "pointer_and_nested_maps",
			value: CompatibilityValues{
				NamedIntNested:      &integerPointer,
				NamedIntPtrSliceMap: map[string][]*NamedInt{"values": {nil, &integer}},
				NestedAddressPtrMap: map[string]map[string]*Address{"outer": {"nil": nil, "value": {Street: "street", City: "city", Zip: "zip"}}},
			},
		},
		{
			name: "sparse_nil",
			value: CompatibilityValues{
				Slice: []int{}, Map: map[string]int{}, Bytes: []byte{}, Raw: json.RawMessage("null"),
				NamedByteSliceMap: map[string][]NamedByte{"nil": nil, "empty": {}},
			},
		},
	}
	for i := range shapes {
		shape := &shapes[i]
		shape.marshal = shape.value.MarshalJSON
		shape.unmarshal = func(data []byte) error {
			var value CompatibilityValues
			return value.UnmarshalJSON(data)
		}
		payload, err := shape.marshal()
		if err != nil {
			b.Fatal(err)
		}
		b.Run(shape.name+"/marshal", func(b *testing.B) {
			b.SetBytes(int64(len(payload)))
			b.ReportAllocs()
			for b.Loop() {
				benchmarkJSONSink, err = shape.marshal()
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(shape.name+"/unmarshal", func(b *testing.B) {
			b.SetBytes(int64(len(payload)))
			b.ReportAllocs()
			for b.Loop() {
				if err := shape.unmarshal(payload); err != nil {
					b.Fatal(err)
				}
			}
		})
	}

	numbers := CompatibilityNumbers{
		Int8: math.MinInt8, Int16: math.MinInt16, Int32: math.MinInt32, Int64: math.MinInt64,
		Uint8: math.MaxUint8, Uint16: math.MaxUint16, Uint32: math.MaxUint32, Uint64: math.MaxUint64,
		Float32: math.SmallestNonzeroFloat32, Float64: math.SmallestNonzeroFloat64,
		Named: math.MaxInt64, NamedUint: math.MaxUint32, NamedFloat: math.MaxFloat32,
	}
	numberPayload, err := numbers.MarshalJSON()
	if err != nil {
		b.Fatal(err)
	}
	b.Run("numeric_boundaries/marshal", func(b *testing.B) {
		b.SetBytes(int64(len(numberPayload)))
		b.ReportAllocs()
		for b.Loop() {
			benchmarkJSONSink, err = numbers.MarshalJSON()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("numeric_boundaries/unmarshal", func(b *testing.B) {
		b.SetBytes(int64(len(numberPayload)))
		b.ReportAllocs()
		for b.Loop() {
			var value CompatibilityNumbers
			if err := value.UnmarshalJSON(numberPayload); err != nil {
				b.Fatal(err)
			}
		}
	})
}
