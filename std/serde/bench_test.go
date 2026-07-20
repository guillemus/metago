package serde

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
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
	return benchPayload(benchFeed(users))
}

func benchPayload(feed Feed) []byte {
	data, err := json.Marshal(toStdBench(feed))
	if err != nil {
		panic(err)
	}
	return data
}

func benchEscapedFeed(users int) Feed {
	feed := benchFeed(users)
	for i := range feed.Users {
		feed.Users[i].Name = strings.Repeat("<&>\\\"\n日本語\u2028", 8)
		feed.Users[i].Email = fmt.Sprintf("用戶+%d@example.com", i)
		feed.Users[i].Tags = []string{"plain", "\tcontrol", "emoji-🚀"}
		feed.Users[i].Address.Street = "Carrer de l'Àliga <42>"
		feed.Users[i].Metadata = map[string]string{"<&>": "日本語", "escaped": "\\\"\n"}
	}
	return feed
}

func benchReorderedUnknownPayload(users int) []byte {
	var document map[string]any
	if err := json.Unmarshal(benchFeedPayload(users), &document); err != nil {
		panic(err)
	}
	for _, value := range document["users"].([]any) {
		user := value.(map[string]any)
		user["_unknown"] = map[string]any{
			"nested": []any{true, nil, map[string]any{"value": "ignored"}},
		}
	}
	document["_top_level_unknown"] = "ignored"
	data, err := json.MarshalIndent(document, "", "  ")
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

type unmarshalBenchmarkCodec struct {
	name string
	fn   func([]byte) error
}

func unmarshalBenchmarkCodecs() []unmarshalBenchmarkCodec {
	return []unmarshalBenchmarkCodec{
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
}

func runUnmarshalBenchmarks(b *testing.B, payload []byte) {
	for _, codec := range unmarshalBenchmarkCodecs() {
		b.Run(codec.name, func(b *testing.B) {
			if err := codec.fn(payload); err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(len(payload)))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := codec.fn(payload); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkUnmarshal(b *testing.B) {
	for _, size := range benchSizes {
		payload := benchFeedPayload(size.users)
		b.Run(size.name, func(b *testing.B) {
			runUnmarshalBenchmarks(b, payload)
		})
	}
}

// BenchmarkUnmarshalInputShapes exercises cases that are less favorable to generated field
// dispatch and arena-backed plain strings than the canonical feed benchmark.
func BenchmarkUnmarshalInputShapes(b *testing.B) {
	shapes := []struct {
		name    string
		payload []byte
	}{
		{"canonical", benchFeedPayload(100)},
		{"escaped_unicode", benchPayload(benchEscapedFeed(100))},
		{"reordered_unknown_indented", benchReorderedUnknownPayload(100)},
	}
	for _, shape := range shapes {
		b.Run(shape.name, func(b *testing.B) {
			runUnmarshalBenchmarks(b, shape.payload)
		})
	}
}

type marshalBenchmarkCodec struct {
	name string
	fn   func() ([]byte, error)
}

func marshalBenchmarkCodecs(b *testing.B, feed Feed) []marshalBenchmarkCodec {
	b.Helper()
	stdFeed := toStdBench(feed)
	var easyFeed EasyFeed
	if err := easyFeed.UnmarshalJSON(benchPayload(feed)); err != nil {
		b.Fatal(err)
	}
	return []marshalBenchmarkCodec{
		{"serde", func() ([]byte, error) { return feed.MarshalJSON() }},
		{"stdlib", func() ([]byte, error) { return json.Marshal(stdFeed) }},
		{"easyjson", func() ([]byte, error) { return easyFeed.MarshalJSON() }},
		{"goccy", func() ([]byte, error) { return gojson.Marshal(stdFeed) }},
		{"jsoniter", func() ([]byte, error) { return jsoniterCompat.Marshal(stdFeed) }},
		{"sonic", func() ([]byte, error) { return sonic.Marshal(stdFeed) }},
	}
}

func runMarshalBenchmarks(b *testing.B, feed Feed) {
	expected := toStdBench(feed)
	for _, codec := range marshalBenchmarkCodecs(b, feed) {
		b.Run(codec.name, func(b *testing.B) {
			encoded, err := codec.fn()
			if err != nil {
				b.Fatal(err)
			}
			var decoded StdFeed
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				b.Fatalf("encoder returned invalid JSON: %v", err)
			}
			if !reflect.DeepEqual(decoded, expected) {
				b.Fatal("encoder output does not match the benchmark value")
			}
			b.SetBytes(int64(len(encoded)))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				benchmarkJSONSink, err = codec.fn()
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkMarshal(b *testing.B) {
	for _, size := range benchSizes {
		b.Run(size.name, func(b *testing.B) {
			runMarshalBenchmarks(b, benchFeed(size.users))
		})
	}
}

// BenchmarkMarshalInputShapes compares escaping and map-ordering work that the canonical feed
// intentionally keeps modest.
func BenchmarkMarshalInputShapes(b *testing.B) {
	mapHeavy := benchFeed(100)
	for i := range mapHeavy.Users {
		mapHeavy.Users[i].Metadata = make(map[string]string, 64)
		for key := range 64 {
			mapHeavy.Users[i].Metadata[fmt.Sprintf("key-%02d", key)] = fmt.Sprintf("value-%d-%d", i, key)
		}
	}
	shapes := []struct {
		name string
		feed Feed
	}{
		{"canonical", benchFeed(100)},
		{"escaped_unicode", benchEscapedFeed(100)},
		{"map_heavy", mapHeavy},
	}
	for _, shape := range shapes {
		b.Run(shape.name, func(b *testing.B) {
			runMarshalBenchmarks(b, shape.feed)
		})
	}
}

// BenchmarkSerdeAPIDispatch makes the direct-method advantage in the comparative benchmarks
// measurable instead of hiding it in the benchmark setup.
func BenchmarkSerdeAPIDispatch(b *testing.B) {
	feed := benchFeed(1)
	payload := benchFeedPayload(1)

	b.Run("marshal/direct", func(b *testing.B) {
		var err error
		b.ReportAllocs()
		for b.Loop() {
			benchmarkJSONSink, err = feed.MarshalJSON()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("marshal/encoding_json", func(b *testing.B) {
		var err error
		b.ReportAllocs()
		for b.Loop() {
			benchmarkJSONSink, err = json.Marshal(feed)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("unmarshal/direct", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			var feed Feed
			if err := feed.UnmarshalJSON(payload); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("unmarshal/encoding_json", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			var feed Feed
			if err := json.Unmarshal(payload, &feed); err != nil {
				b.Fatal(err)
			}
		}
	})
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
