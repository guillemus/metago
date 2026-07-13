package serde

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"strconv"
	"testing"
)

type feedJSON struct {
	Users []userJSON `json:"users"`
}

type userJSON struct {
	ID       int64             `json:"id"`
	Name     string            `json:"name"`
	Email    string            `json:"email"`
	Age      int               `json:"age"`
	Active   bool              `json:"active"`
	Score    float64           `json:"score"`
	Tags     []string          `json:"tags"`
	Address  addressJSON       `json:"address"`
	Items    []itemJSON        `json:"items"`
	Metadata map[string]string `json:"metadata"`
}

type addressJSON struct {
	Street string `json:"street"`
	City   string `json:"city"`
	Zip    string `json:"zip"`
}

type itemJSON struct {
	SKU   string  `json:"sku"`
	Qty   int     `json:"qty"`
	Price float64 `json:"price"`
}

func TestGeneratedSliceArenaGrowthKeepsEarlierSlices(t *testing.T) {
	var arena serdeJSONSliceArena[string]
	first := arena.take(4, 400, 100)
	first = append(first, "a", "b", "c", "d")
	second := arena.take(4, 400, 100)
	second = append(second, "e", "f", "g", "h")

	if want := []string{"a", "b", "c", "d"}; !reflect.DeepEqual(first, want) {
		t.Fatalf("arena growth changed an earlier slice: got %#v, want %#v", first, want)
	}
	if want := []string{"e", "f", "g", "h"}; !reflect.DeepEqual(second, want) {
		t.Fatalf("later arena slice mismatch: got %#v, want %#v", second, want)
	}
}

func TestGeneratedNestedSlicesMatchEncodingJSON(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	counts := []int{0, 1, 4, 5, 8, 9, 16, 33}
	for range 128 {
		counts = append(counts, rng.Intn(34))
	}

	for caseIndex, userCount := range counts {
		want := feedJSON{Users: make([]userJSON, userCount)}
		for userIndex := range want.Users {
			tagCount := counts[(caseIndex+userIndex)%len(counts)]
			itemCount := counts[(caseIndex*3+userIndex)%len(counts)]
			user := &want.Users[userIndex]
			user.ID = int64(caseIndex*100 + userIndex)
			user.Name = nestedTestString(userIndex)
			user.Email = nestedTestString(caseIndex)
			user.Age = userIndex
			user.Active = userIndex%2 == 0
			user.Score = float64(userIndex) + 0.25
			user.Address = addressJSON{Street: nestedTestString(userIndex + 1), City: "Asunción", Zip: strconv.Itoa(userIndex)}
			user.Metadata = map[string]string{"case": strconv.Itoa(caseIndex), "user": strconv.Itoa(userIndex)}
			user.Tags = make([]string, tagCount)
			for i := range user.Tags {
				user.Tags[i] = nestedTestString(i)
			}
			user.Items = make([]itemJSON, itemCount)
			for i := range user.Items {
				user.Items[i] = itemJSON{SKU: nestedTestString(i), Qty: i, Price: float64(i) + 0.5}
			}
		}

		data, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		var decoded Feed
		if err := decoded.UnmarshalJSON(data); err != nil {
			t.Fatalf("case %d: %v", caseIndex, err)
		}
		if got := feedJSONFromGenerated(decoded); !reflect.DeepEqual(got, want) {
			t.Fatalf("case %d differs from encoding/json:\n got: %#v\nwant: %#v", caseIndex, got, want)
		}
	}
}

func TestGeneratedNestedSlicesHandleDuplicateFields(t *testing.T) {
	input := []byte(`{"users":[{
		"tags":["old"],"tags":["new","last"],
		"items":[{"sku":"old"}],"items":[{"sku":"new","qty":2}],
		"metadata":{"first":"1","same":"old"},
		"metadata":{"second":"2","same":"new"}
	}]}`)

	var got Feed
	if err := got.UnmarshalJSON(input); err != nil {
		t.Fatal(err)
	}
	want := Feed{Users: []User{{
		Tags:  []string{"new", "last"},
		Items: []Item{{SKU: "new", Qty: 2}},
		Metadata: map[string]string{
			"first": "1", "second": "2", "same": "new",
		},
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("duplicate nested fields mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func nestedTestString(index int) string {
	return "value-" + strconv.Itoa(index) + "-\n-日本語-\\-\""
}

func feedJSONFromGenerated(value Feed) feedJSON {
	result := feedJSON{Users: make([]userJSON, len(value.Users))}
	for i, user := range value.Users {
		converted := &result.Users[i]
		converted.ID = user.ID
		converted.Name = user.Name
		converted.Email = user.Email
		converted.Age = user.Age
		converted.Active = user.Active
		converted.Score = user.Score
		converted.Tags = user.Tags
		converted.Address = addressJSON(user.Address)
		converted.Metadata = user.Metadata
		converted.Items = make([]itemJSON, len(user.Items))
		for itemIndex, item := range user.Items {
			converted.Items[itemIndex] = itemJSON(item)
		}
	}
	return result
}
