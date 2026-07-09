package jsonexp

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/bytedance/sonic"
	gojson "github.com/goccy/go-json"
	jsoniter "github.com/json-iterator/go"
)

var jsoniterCompat = jsoniter.ConfigCompatibleWithStandardLibrary

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
			{"metago", func(d []byte) error {
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
			{"metago", func() ([]byte, error) { return feed.MarshalJSON() }},
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
