package gocb

import (
	"context"
	"reflect"
	"testing"
	"time"

	gocbcore "github.com/couchbase/gocbcore/v8"
)

func TestErrorNonExistant(t *testing.T) {
	res, err := globalCollection.Get("doesnt-exist", nil)
	if err == nil {
		t.Fatalf("Expected error to be non-nil")
	}

	if res != nil {
		t.Fatalf("Expected result to be nil but was %v", res)
	}
}

func TestErrorDoubleInsert(t *testing.T) {
	_, err := globalCollection.Insert("doubleInsert", "test", nil)
	if err != nil {
		t.Fatalf("Expected error to be nil but was %v", err)
	}
	_, err = globalCollection.Insert("doubleInsert", "test", nil)
	if err == nil {
		t.Fatalf("Expected error to be non-nil")
	}

	if !IsKeyExistsError(err) {
		t.Fatalf("Expected error to be KeyExistsError but is %s", reflect.TypeOf(err).String())
	}
}

func TestInsertGetWithExpiry(t *testing.T) {
	if globalCluster.NotSupportsFeature(XattrFeature) {
		t.Skip("Skipping test as xattrs not supported.")
	}

	var doc testBeerDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not read test dataset: %v", err)
	}

	mutRes, err := globalCollection.Insert("expiryDoc", doc, &InsertOptions{Expiration: 10})
	if err != nil {
		t.Fatalf("Insert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Insert CAS was 0")
	}

	insertedDoc, err := globalCollection.Get("expiryDoc", &GetOptions{WithExpiry: true})
	if err != nil {
		t.Fatalf("Get failed, error was %v", err)
	}

	var insertedDocContent testBeerDocument
	err = insertedDoc.Content(&insertedDocContent)
	if err != nil {
		t.Fatalf("Content failed, error was %v", err)
	}

	if doc != insertedDocContent {
		t.Fatalf("Expected resulting doc to be %v but was %v", doc, insertedDocContent)
	}

	if !insertedDoc.HasExpiration() {
		t.Fatalf("Expected document to have an expiry")
	}

	if insertedDoc.Expiration() == 0 {
		t.Fatalf("Expected expiry value to be populated")
	}
}

func TestInsertGetProjection(t *testing.T) {
	type PersonDimensions struct {
		Height int `json:"height"`
		Weight int `json:"weight"`
	}
	type Location struct {
		Lat  float32 `json:"lat"`
		Long float32 `json:"long"`
	}
	type HobbyDetails struct {
		Location Location `json:"location"`
	}
	type PersonHobbies struct {
		Type    string       `json:"type"`
		Name    string       `json:"name"`
		Details HobbyDetails `json:"details,omitempty"`
	}
	type PersonAttributes struct {
		Hair       string           `json:"hair"`
		Dimensions PersonDimensions `json:"dimensions"`
		Hobbies    []PersonHobbies  `json:"hobbies"`
	}
	type Person struct {
		Name       string           `json:"name"`
		Age        int              `json:"age"`
		Animals    []string         `json:"animals"`
		Attributes PersonAttributes `json:"attributes"`
	}

	var person Person
	err := loadJSONTestDataset("projection_doc", &person)
	if err != nil {
		t.Fatalf("Could not read test dataset: %v", err)
	}

	mutRes, err := globalCollection.Upsert("projectDoc", person, nil)
	if err != nil {
		t.Fatalf("Insert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Insert CAS was 0")
	}

	type tCase struct {
		name     string
		project  []string
		expected Person
	}

	testCases := []tCase{
		{
			name:     "string",
			project:  []string{"name"},
			expected: Person{Name: person.Name},
		},
		{
			name:     "int",
			project:  []string{"age"},
			expected: Person{Age: person.Age},
		},
		{
			name:     "array",
			project:  []string{"animals"},
			expected: Person{Animals: person.Animals},
		},
		{
			name:     "array-index1",
			project:  []string{"animals[0]"},
			expected: Person{Animals: []string{person.Animals[0]}},
		},
		{
			name:     "array-index2",
			project:  []string{"animals[1]"},
			expected: Person{Animals: []string{person.Animals[1]}},
		},
		{
			name:     "array-index3",
			project:  []string{"animals[2]"},
			expected: Person{Animals: []string{person.Animals[2]}},
		},
		{
			name:     "full-object-field",
			project:  []string{"attributes"},
			expected: Person{Attributes: person.Attributes},
		},
		{
			name:    "nested-object-field1",
			project: []string{"attributes.hair"},
			expected: Person{
				Attributes: PersonAttributes{
					Hair: person.Attributes.Hair,
				},
			},
		},
		{
			name:    "nested-object-field2",
			project: []string{"attributes.dimensions"},
			expected: Person{
				Attributes: PersonAttributes{
					Dimensions: person.Attributes.Dimensions,
				},
			},
		},
		{
			name:    "nested-object-field3",
			project: []string{"attributes.dimensions.height"},
			expected: Person{
				Attributes: PersonAttributes{
					Dimensions: PersonDimensions{
						Height: person.Attributes.Dimensions.Height,
					},
				},
			},
		},
		{
			name:    "nested-object-field3",
			project: []string{"attributes.dimensions.weight"},
			expected: Person{
				Attributes: PersonAttributes{
					Dimensions: PersonDimensions{
						Weight: person.Attributes.Dimensions.Weight,
					},
				},
			},
		},
		{
			name:    "nested-object-field4",
			project: []string{"attributes.hobbies"},
			expected: Person{
				Attributes: PersonAttributes{
					Hobbies: person.Attributes.Hobbies,
				},
			},
		},
		{
			name:    "nested-array-object-field1",
			project: []string{"attributes.hobbies[0].type"},
			expected: Person{
				Attributes: PersonAttributes{
					Hobbies: []PersonHobbies{
						PersonHobbies{
							Type: person.Attributes.Hobbies[0].Type,
						},
					},
				},
			},
		},
		{
			name:    "nested-array-object-field2",
			project: []string{"attributes.hobbies[1].type"},
			expected: Person{
				Attributes: PersonAttributes{
					Hobbies: []PersonHobbies{
						PersonHobbies{
							Type: person.Attributes.Hobbies[1].Type,
						},
					},
				},
			},
		},
		{
			name:    "nested-array-object-field3",
			project: []string{"attributes.hobbies[0].name"},
			expected: Person{
				Attributes: PersonAttributes{
					Hobbies: []PersonHobbies{
						PersonHobbies{
							Name: person.Attributes.Hobbies[0].Name,
						},
					},
				},
			},
		},
		{
			name:    "nested-array-object-field4",
			project: []string{"attributes.hobbies[1].name"},
			expected: Person{
				Attributes: PersonAttributes{
					Hobbies: []PersonHobbies{
						PersonHobbies{
							Name: person.Attributes.Hobbies[1].Name,
						},
					},
				},
			},
		},
		{
			name:    "nested-array-object-field5",
			project: []string{"attributes.hobbies[1].details"},
			expected: Person{
				Attributes: PersonAttributes{
					Hobbies: []PersonHobbies{
						PersonHobbies{
							Details: person.Attributes.Hobbies[1].Details,
						},
					},
				},
			},
		},
	}

	// a bug in Mock prevents these cases from working correctly
	if globalCluster.SupportsFeature(SubdocMockBugFeature) {
		testCases = append(testCases,
			tCase{
				name:    "nested-array-object-nested-field1",
				project: []string{"attributes.hobbies[1].details.location"},
				expected: Person{
					Attributes: PersonAttributes{
						Hobbies: []PersonHobbies{
							PersonHobbies{
								Details: HobbyDetails{
									Location: person.Attributes.Hobbies[1].Details.Location,
								},
							},
						},
					},
				},
			},
			tCase{
				name:    "nested-array-object-nested-nested-field1",
				project: []string{"attributes.hobbies[1].details.location.lat"},
				expected: Person{
					Attributes: PersonAttributes{
						Hobbies: []PersonHobbies{
							PersonHobbies{
								Details: HobbyDetails{
									Location: Location{
										Lat: person.Attributes.Hobbies[1].Details.Location.Lat,
									},
								},
							},
						},
					},
				},
			},
			tCase{
				name:    "nested-array-object-nested-nested-field2",
				project: []string{"attributes.hobbies[1].details.location.long"},
				expected: Person{
					Attributes: PersonAttributes{
						Hobbies: []PersonHobbies{
							PersonHobbies{
								Details: HobbyDetails{
									Location: Location{
										Long: person.Attributes.Hobbies[1].Details.Location.Long,
									},
								},
							},
						},
					},
				},
			},
		)
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			doc, err := globalCollection.Get("projectDoc", &GetOptions{
				Project: &ProjectOptions{
					Fields: testCase.project,
				},
			})
			if err != nil {
				t.Fatalf("Get failed, error was %v", err)
			}

			var actual Person
			err = doc.Content(&actual)
			if err != nil {
				t.Fatalf("Content failed, error was %v", err)
			}

			if !reflect.DeepEqual(actual, testCase.expected) {
				t.Fatalf("Projection failed, expected %+v but was %+v", testCase.expected, actual)
			}
		})
	}
}

func TestInsertGetProjection17Fields(t *testing.T) {
	var doc testBeerDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not read test dataset: %v", err)
	}

	mutRes, err := globalCollection.Insert("projectDocTooManyFields", doc, nil)
	if err != nil {
		t.Fatalf("Insert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Insert CAS was 0")
	}

	insertedDoc, err := globalCollection.Get("projectDocTooManyFields", &GetOptions{
		Project: &ProjectOptions{
			Fields: []string{"field1", "field2", "field3", "field4", "field5", "field6", "field7", "field8", "field9",
				"field1", "field10", "field12", "field13", "field14", "field15", "field16", "field17"},
		},
	})
	if err != nil {
		t.Fatalf("Get failed, error was %v", err)
	}

	var insertedDocContent testBeerDocument
	err = insertedDoc.Content(&insertedDocContent)
	if err != nil {
		t.Fatalf("Content failed, error was %v", err)
	}

	if insertedDocContent != doc {
		t.Fatalf("Expected resulting doc to be %v but was %v", doc, insertedDocContent)
	}
}

func TestInsertGetProjection16FieldsExpiry(t *testing.T) {
	if globalCluster.NotSupportsFeature(XattrFeature) {
		t.Skip("Skipping test as xattrs not supported.")
	}

	var doc testBeerDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not read test dataset: %v", err)
	}

	mutRes, err := globalCollection.Upsert("projectDocTooManyFieldsExpiry", doc, &UpsertOptions{
		Expiration: 60,
	})
	if err != nil {
		t.Fatalf("Insert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Insert CAS was 0")
	}

	insertedDoc, err := globalCollection.Get("projectDocTooManyFieldsExpiry", &GetOptions{
		Project: &ProjectOptions{
			Fields: []string{"field1", "field2", "field3", "field4", "field5", "field6", "field7", "field8", "field9",
				"field1", "field10", "field12", "field13", "field14", "field15", "field16"},
		},
		WithExpiry: true,
	})
	if err != nil {
		t.Fatalf("Get failed, error was %v", err)
	}

	var insertedDocContent testBeerDocument
	err = insertedDoc.Content(&insertedDocContent)
	if err != nil {
		t.Fatalf("Content failed, error was %v", err)
	}

	if insertedDocContent != doc {
		t.Fatalf("Expected resulting doc to be %v but was %v", doc, insertedDocContent)
	}

	if !insertedDoc.HasExpiration() {
		t.Fatalf("Expected document to have an expiry")
	}

	if insertedDoc.Expiration() == 0 {
		t.Fatalf("Expected expiry value to be populated")
	}
}

func TestInsertGetProjectionPathMissing(t *testing.T) {
	var doc testBeerDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not read test dataset: %v", err)
	}

	mutRes, err := globalCollection.Insert("projectMissingDoc", doc, nil)
	if err != nil {
		t.Fatalf("Insert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Insert CAS was 0")
	}

	_, err = globalCollection.Get("projectMissingDoc", &GetOptions{
		Project: &ProjectOptions{
			Fields:                 []string{"name", "thisfielddoesntexist"},
			IgnorePathMissingError: false,
		},
	})
	if err == nil {
		t.Fatalf("Get should have failed")
	}
}

func TestInsertGetProjectionIgnorePathMissing(t *testing.T) {
	var doc testBeerDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not read test dataset: %v", err)
	}

	mutRes, err := globalCollection.Insert("projectIgnoreMissingDoc", doc, nil)
	if err != nil {
		t.Fatalf("Insert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Insert CAS was 0")
	}

	insertedDoc, err := globalCollection.Get("projectIgnoreMissingDoc", &GetOptions{
		Project: &ProjectOptions{
			Fields:                 []string{"name", "thisfielddoesntexist"},
			IgnorePathMissingError: true,
		},
	})
	if err != nil {
		t.Fatalf("Get failed, error was %v", err)
	}

	var insertedDocContent testBeerDocument
	err = insertedDoc.Content(&insertedDocContent)
	if err != nil {
		t.Fatalf("Content failed, error was %v", err)
	}

	expectedDoc := testBeerDocument{
		Name: doc.Name,
	}

	if insertedDocContent != expectedDoc {
		t.Fatalf("Expected resulting doc to be %v but was %v", expectedDoc, insertedDocContent)
	}
}

func TestInsertGet(t *testing.T) {
	var doc testBeerDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not read test dataset: %v", err)
	}

	mutRes, err := globalCollection.Insert("insertDoc", doc, nil)
	if err != nil {
		t.Fatalf("Insert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Insert CAS was 0")
	}

	insertedDoc, err := globalCollection.Get("insertDoc", nil)
	if err != nil {
		t.Fatalf("Get failed, error was %v", err)
	}

	var insertedDocContent testBeerDocument
	err = insertedDoc.Content(&insertedDocContent)
	if err != nil {
		t.Fatalf("Content failed, error was %v", err)
	}

	if doc != insertedDocContent {
		t.Fatalf("Expected resulting doc to be %v but was %v", doc, insertedDocContent)
	}
}

// func TestInsertGetRetryInsert(t *testing.T) {
// 	if testing.Short() {
// 		t.Skip("Skipping test in short mode.")
// 	}
//
// 	if globalCluster.NotSupportsFeature(CollectionsFeature) {
// 		t.Skip("Skipping test as collections not supported.")
// 	}
//
// 	var doc testBeerDocument
// 	err := loadJSONTestDataset("beer_sample_single", &doc)
// 	if err != nil {
// 		t.Fatalf("Could not read test dataset: %v", err)
// 	}
//
// 	cli := globalBucket.sb.getCachedClient()
// 	_, err = testCreateCollection("insertRetry", "_default", globalBucket, cli)
// 	if err != nil {
// 		t.Fatalf("Failed to create collection, error was %v", err)
// 	}
// 	defer testDeleteCollection("insertRetry", "_default", globalBucket, cli, true)
//
// 	col := globalBucket.Collection("_default", "insertRetry", nil)
//
// 	_, err = testDeleteCollection("insertRetry", "_default", globalBucket, cli, true)
// 	if err != nil {
// 		t.Fatalf("Failed to delete collection, error was %v", err)
// 	}
//
// 	_, err = testCreateCollection("insertRetry", "_default", globalBucket, cli)
// 	if err != nil {
// 		t.Fatalf("Failed to create collection, error was %v", err)
// 	}
//
// 	mutRes, err := col.Insert("nonDefaultInsertDoc", doc, nil)
// 	if err != nil {
// 		t.Fatalf("Insert failed, error was %v", err)
// 	}
//
// 	if mutRes.Cas() == 0 {
// 		t.Fatalf("Insert CAS was 0")
// 	}
//
// 	insertedDoc, err := col.Get("nonDefaultInsertDoc", nil)
// 	if err != nil {
// 		t.Fatalf("Get failed, error was %v", err)
// 	}
//
// 	var insertedDocContent testBeerDocument
// 	err = insertedDoc.Content(&insertedDocContent)
// 	if err != nil {
// 		t.Fatalf("Content failed, error was %v", err)
// 	}
//
// 	if doc != insertedDocContent {
// 		t.Fatalf("Expected resulting doc to be %v but was %v", doc, insertedDocContent)
// 	}
// }

func TestUpsertGetRemove(t *testing.T) {
	var doc testBeerDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not read test dataset: %v", err)
	}

	mutRes, err := globalCollection.Upsert("upsertDoc", doc, nil)
	if err != nil {
		t.Fatalf("Upsert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Upsert CAS was 0")
	}

	upsertedDoc, err := globalCollection.Get("upsertDoc", nil)
	if err != nil {
		t.Fatalf("Get failed, error was %v", err)
	}

	var upsertedDocContent testBeerDocument
	err = upsertedDoc.Content(&upsertedDocContent)
	if err != nil {
		t.Fatalf("Content failed, error was %v", err)
	}

	if doc != upsertedDocContent {
		t.Fatalf("Expected resulting doc to be %v but was %v", doc, upsertedDocContent)
	}

	existsRes, err := globalCollection.Exists("upsertDoc", nil)
	if err != nil {
		t.Fatalf("Exists failed, error was %v", err)
	}

	if !existsRes.Exists() {
		t.Fatalf("Expected exists to return true")
	}

	_, err = globalCollection.Remove("upsertDoc", nil)
	if err != nil {
		t.Fatalf("Remove failed, error was %v", err)
	}

	existsRes, err = globalCollection.Exists("upsertDoc", nil)
	if err != nil {
		t.Fatalf("Exists failed, error was %v", err)
	}

	if existsRes.Exists() {
		t.Fatalf("Expected exists to return false")
	}
}

func TestRemoveWithCas(t *testing.T) {
	var doc testBeerDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not read test dataset: %v", err)
	}

	mutRes, err := globalCollection.Upsert("removeWithCas", doc, nil)
	if err != nil {
		t.Fatalf("Upsert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Upsert CAS was 0")
	}

	existsRes, err := globalCollection.Exists("removeWithCas", nil)
	if err != nil {
		t.Fatalf("Exists failed, error was %v", err)
	}

	if !existsRes.Exists() {
		t.Fatalf("Expected exists to return true")
	}

	_, err = globalCollection.Remove("removeWithCas", &RemoveOptions{Cas: mutRes.Cas() + 0xFECA})
	if err == nil {
		t.Fatalf("Expected remove to fail")
	}

	if !IsKeyExistsError(err) {
		t.Fatalf("Expected error to be KeyExistsError but is %s", reflect.TypeOf(err).String())
	}

	existsRes, err = globalCollection.Exists("removeWithCas", nil)
	if err != nil {
		t.Fatalf("Exists failed, error was %v", err)
	}

	if !existsRes.Exists() {
		t.Fatalf("Expected exists to return true")
	}

	_, err = globalCollection.Remove("removeWithCas", &RemoveOptions{Cas: mutRes.Cas()})
	if err != nil {
		t.Fatalf("Remove failed, error was %v", err)
	}

	existsRes, err = globalCollection.Exists("removeWithCas", nil)
	if err != nil {
		t.Fatalf("Exists failed, error was %v", err)
	}

	if existsRes.Exists() {
		t.Fatalf("Expected exists to return false")
	}
}

func TestUpsertAndReplace(t *testing.T) {
	var doc testBeerDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not read test dataset: %v", err)
	}

	mutRes, err := globalCollection.Upsert("upsertAndReplace", doc, nil)
	if err != nil {
		t.Fatalf("Upsert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Upsert CAS was 0")
	}

	insertedDoc, err := globalCollection.Get("upsertAndReplace", nil)
	if err != nil {
		t.Fatalf("Get failed, error was %v", err)
	}

	var insertedDocContent testBeerDocument
	err = insertedDoc.Content(&insertedDocContent)
	if err != nil {
		t.Fatalf("Content failed, error was %v", err)
	}

	if doc != insertedDocContent {
		t.Fatalf("Expected resulting doc to be %v but was %v", doc, insertedDocContent)
	}

	doc.Name = "replaced"
	mutRes, err = globalCollection.Replace("upsertAndReplace", doc, &ReplaceOptions{Cas: mutRes.Cas()})
	if err != nil {
		t.Fatalf("Upsert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Upsert CAS was 0")
	}

	replacedDoc, err := globalCollection.Get("upsertAndReplace", nil)
	if err != nil {
		t.Fatalf("Get failed, error was %v", err)
	}

	var replacedDocContent testBeerDocument
	err = replacedDoc.Content(&replacedDocContent)
	if err != nil {
		t.Fatalf("Content failed, error was %v", err)
	}

	if doc != replacedDocContent {
		t.Fatalf("Expected resulting doc to be %v but was %v", doc, insertedDocContent)
	}
}

func TestGetAndTouch(t *testing.T) {
	if globalCluster.NotSupportsFeature(XattrFeature) {
		t.Skip("Skipping test as xattrs not supported.")
	}

	var doc testBeerDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not read test dataset: %v", err)
	}

	mutRes, err := globalCollection.Upsert("getAndTouch", doc, nil)
	if err != nil {
		t.Fatalf("Upsert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Upsert CAS was 0")
	}

	lockedDoc, err := globalCollection.GetAndTouch("getAndTouch", 10, nil)
	if err != nil {
		t.Fatalf("Get failed, error was %v", err)
	}

	var lockedDocContent testBeerDocument
	err = lockedDoc.Content(&lockedDocContent)
	if err != nil {
		t.Fatalf("Content failed, error was %v", err)
	}

	if doc != lockedDocContent {
		t.Fatalf("Expected resulting doc to be %v but was %v", doc, lockedDocContent)
	}

	expireDoc, err := globalCollection.Get("getAndTouch", &GetOptions{WithExpiry: true})
	if err != nil {
		t.Fatalf("Get failed, error was %v", err)
	}

	if !expireDoc.HasExpiration() {
		t.Fatalf("Expected doc to have an expiry")
	}

	if expireDoc.Expiration() == 0 {
		t.Fatalf("Expected doc to have an expiry > 0, was %d", expireDoc.Expiration())
	}

	var expireDocContent testBeerDocument
	err = expireDoc.Content(&expireDocContent)
	if err != nil {
		t.Fatalf("Content failed, error was %v", err)
	}

	if doc != expireDocContent {
		t.Fatalf("Expected resulting doc to be %v but was %v", doc, lockedDocContent)
	}
}

func TestGetAndLock(t *testing.T) {
	var doc testBeerDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not read test dataset: %v", err)
	}

	mutRes, err := globalCollection.Upsert("getAndLock", doc, nil)
	if err != nil {
		t.Fatalf("Upsert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Upsert CAS was 0")
	}

	lockedDoc, err := globalCollection.GetAndLock("getAndLock", 1, nil)
	if err != nil {
		t.Fatalf("Get failed, error was %v", err)
	}

	var lockedDocContent testBeerDocument
	err = lockedDoc.Content(&lockedDocContent)
	if err != nil {
		t.Fatalf("Content failed, error was %v", err)
	}

	if doc != lockedDocContent {
		t.Fatalf("Expected resulting doc to be %v but was %v", doc, lockedDocContent)
	}

	mutRes, err = globalCollection.Upsert("getAndLock", doc, nil)
	if err == nil {
		t.Fatalf("Expected error but was nil")
	}

	if !IsKeyExistsError(err) {
		t.Fatalf("Expected error to be KeyExistsError but is %s", reflect.TypeOf(err).String())
	}

	globalCluster.TimeTravel(2000 * time.Millisecond)

	mutRes, err = globalCollection.Upsert("getAndLock", doc, nil)
	if err != nil {
		t.Fatalf("Upsert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Upsert CAS was 0")
	}
}

func TestUnlock(t *testing.T) {
	var doc testBeerDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not read test dataset: %v", err)
	}

	mutRes, err := globalCollection.Upsert("unlock", doc, nil)
	if err != nil {
		t.Fatalf("Upsert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Upsert CAS was 0")
	}

	lockedDoc, err := globalCollection.GetAndLock("unlock", 1, nil)
	if err != nil {
		t.Fatalf("Get failed, error was %v", err)
	}

	var lockedDocContent testBeerDocument
	err = lockedDoc.Content(&lockedDocContent)
	if err != nil {
		t.Fatalf("Content failed, error was %v", err)
	}

	if doc != lockedDocContent {
		t.Fatalf("Expected resulting doc to be %v but was %v", doc, lockedDocContent)
	}

	_, err = globalCollection.Unlock("unlock", &UnlockOptions{Cas: lockedDoc.Cas()})
	if err != nil {
		t.Fatalf("Unlock failed, error was %v", err)
	}

	mutRes, err = globalCollection.Upsert("unlock", doc, nil)
	if err != nil {
		t.Fatalf("Upsert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Upsert CAS was 0")
	}
}

func TestUnlockInvalidCas(t *testing.T) {
	var doc testBeerDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not read test dataset: %v", err)
	}

	mutRes, err := globalCollection.Upsert("unlockInvalidCas", doc, nil)
	if err != nil {
		t.Fatalf("Upsert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Upsert CAS was 0")
	}

	lockedDoc, err := globalCollection.GetAndLock("unlockInvalidCas", 1, nil)
	if err != nil {
		t.Fatalf("Get failed, error was %v", err)
	}

	var lockedDocContent testBeerDocument
	err = lockedDoc.Content(&lockedDocContent)
	if err != nil {
		t.Fatalf("Content failed, error was %v", err)
	}

	if doc != lockedDocContent {
		t.Fatalf("Expected resulting doc to be %v but was %v", doc, lockedDocContent)
	}

	_, err = globalCollection.Unlock("unlockInvalidCas", &UnlockOptions{Cas: lockedDoc.Cas() + 1})
	if err == nil {
		t.Fatalf("Unlock should have failed")
	}

	if !IsTemporaryFailureError(err) {
		t.Fatalf("Expected error to be TempFailError but was %s", reflect.TypeOf(err).String())
	}
}

func TestDoubleLockFail(t *testing.T) {
	var doc testBeerDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not read test dataset: %v", err)
	}

	mutRes, err := globalCollection.Upsert("doubleLock", doc, nil)
	if err != nil {
		t.Fatalf("Upsert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Upsert CAS was 0")
	}

	lockedDoc, err := globalCollection.GetAndLock("doubleLock", 1, nil)
	if err != nil {
		t.Fatalf("Get failed, error was %v", err)
	}

	var lockedDocContent testBeerDocument
	err = lockedDoc.Content(&lockedDocContent)
	if err != nil {
		t.Fatalf("Content failed, error was %v", err)
	}

	if doc != lockedDocContent {
		t.Fatalf("Expected resulting doc to be %v but was %v", doc, lockedDocContent)
	}

	_, err = globalCollection.GetAndLock("doubleLock", 1, nil)
	if err == nil {
		t.Fatalf("Expected GetAndLock to fail")
	}

	if !IsTemporaryFailureError(err) {
		t.Fatalf("Expected error to be TempFailError but was %v", err)
	}
}

func TestUnlockMissingDocFail(t *testing.T) {
	_, err := globalCollection.Unlock("unlockMissing", &UnlockOptions{Cas: 123})
	if err == nil {
		t.Fatalf("Expected Unlock to fail")
	}

	if !IsKeyNotFoundError(err) {
		t.Fatalf("Expected error to be KeyNotFoundError but was %v", err)
	}
}

func TestTouch(t *testing.T) {
	if globalCluster.NotSupportsFeature(XattrFeature) {
		t.Skip("Skipping test as xattrs not supported.")
	}

	var doc testBeerDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not read test dataset: %v", err)
	}

	mutRes, err := globalCollection.Upsert("touch", doc, nil)
	if err != nil {
		t.Fatalf("Upsert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Upsert CAS was 0")
	}

	lockedDoc, err := globalCollection.GetAndTouch("touch", 2, nil)
	if err != nil {
		t.Fatalf("Get failed, error was %v", err)
	}

	var lockedDocContent testBeerDocument
	err = lockedDoc.Content(&lockedDocContent)
	if err != nil {
		t.Fatalf("Content failed, error was %v", err)
	}

	if doc != lockedDocContent {
		t.Fatalf("Expected resulting doc to be %v but was %v", doc, lockedDocContent)
	}

	globalCluster.TimeTravel(1 * time.Second)

	touchOut, err := globalCollection.Touch("touch", 3, nil)
	if err != nil {
		t.Fatalf("Touch failed, error was %v", err)
	}

	if touchOut.Cas() == 0 {
		t.Fatalf("Upsert CAS was 0")
	}

	globalCluster.TimeTravel(2 * time.Second)

	expireDoc, err := globalCollection.Get("touch", &GetOptions{WithExpiry: true})
	if err != nil {
		t.Fatalf("Get failed, error was %v", err)
	}

	if !expireDoc.HasExpiration() {
		t.Fatalf("Expected doc to have an expiry")
	}

	if expireDoc.Expiration() == 0 {
		t.Fatalf("Expected doc to have an expiry > 0, was %d", expireDoc.Expiration())
	}

	var expireDocContent testBeerDocument
	err = expireDoc.Content(&expireDocContent)
	if err != nil {
		t.Fatalf("Content failed, error was %v", err)
	}

	if doc != expireDocContent {
		t.Fatalf("Expected resulting doc to be %v but was %v", doc, lockedDocContent)
	}
}

func TestTouchMissingDocFail(t *testing.T) {
	_, err := globalCollection.Touch("touchMissing", 3, nil)
	if err == nil {
		t.Fatalf("Touch should have failed")
	}

	if !IsKeyNotFoundError(err) {
		t.Fatalf("Expected error to be KeyNotFoundError but was %v", err)
	}
}

func TestInsertReplicateToGetAnyReplica(t *testing.T) {
	var doc testBeerDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not read test dataset: %v", err)
	}

	mutRes, err := globalCollection.Insert("insertReplicaDoc", doc, &InsertOptions{
		PersistTo: 1,
	})
	if err != nil {
		t.Fatalf("Insert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Insert CAS was 0")
	}

	insertedDoc, err := globalCollection.GetAnyReplica("insertReplicaDoc", nil)
	if err != nil {
		t.Fatalf("GetFromReplica failed, error was %v", err)
	}

	var insertedDocContent testBeerDocument
	err = insertedDoc.Content(&insertedDocContent)
	if err != nil {
		t.Fatalf("Content failed, error was %v", err)
	}

	if doc != insertedDocContent {
		t.Fatalf("Expected resulting doc to be %v but was %v", doc, insertedDocContent)
	}
}

func TestInsertReplicateToGetAllReplicas(t *testing.T) {
	var doc testBeerDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not read test dataset: %v", err)
	}

	mutRes, err := globalCollection.Upsert("insertAllReplicaDoc", doc, &UpsertOptions{
		PersistTo: 1,
	})
	if err != nil {
		t.Fatalf("Upsert failed, error was %v", err)
	}

	if mutRes.Cas() == 0 {
		t.Fatalf("Insert CAS was 0")
	}

	stream, err := globalCollection.GetAllReplicas("insertAllReplicaDoc", &GetFromReplicaOptions{
		Timeout: 25 * time.Second,
	})
	if err != nil {
		t.Fatalf("GetAllReplicas failed, error was %v", err)
	}

	agent, err := globalCollection.getKvProvider()
	if err != nil {
		t.Fatalf("Failed to get kv provider, was %v", err)
	}

	expectedReplicas := agent.NumReplicas() + 1
	actualReplicas := 0
	numMasters := 0

	var insertedDoc GetReplicaResult
	for stream.Next(&insertedDoc) {
		actualReplicas++

		if insertedDoc.IsMaster() {
			numMasters++
		}

		var insertedDocContent testBeerDocument
		err = insertedDoc.Content(&insertedDocContent)
		if err != nil {
			t.Fatalf("Content failed, error was %v", err)
		}

		if doc != insertedDocContent {
			t.Fatalf("Expected resulting doc to be %v but was %v", doc, insertedDocContent)
		}
	}

	err = stream.Close()
	if err != nil {
		t.Fatalf("Expected stream close to not error, was %v", err)
	}

	if expectedReplicas != actualReplicas {
		t.Fatalf("Expected replicas to be %d but was %d", expectedReplicas, actualReplicas)
	}

	if numMasters != 1 {
		t.Fatalf("Expected number of masters to be 1 but was %d", numMasters)
	}
}

func TestDurabilityTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2000*time.Millisecond)
	defer cancel()
	level := DurabilityLevelMajority

	coerced, timeout := globalCollection.durabilityTimeout(ctx, level)
	if timeout != 1799 { // 1800 minus a bit for the time it takes to get to the calculation
		t.Fatalf("Timeout value should have been %d but was %d", 1799, timeout)
	}

	if coerced {
		t.Fatalf("Expected coerced to be false")
	}
}

func TestDurabilityTimeoutCoerce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
	defer cancel()
	level := DurabilityLevelMajority

	coerced, timeout := globalCollection.durabilityTimeout(ctx, level)
	if timeout != persistenceTimeoutFloor {
		t.Fatalf("Timeout value should have been %d but was %d", persistenceTimeoutFloor, timeout)
	}

	if !coerced {
		t.Fatalf("Expected coerced to be true")
	}
}

func TestDurabilityGetFromAnyReplica(t *testing.T) {
	if !globalCluster.SupportsFeature(DurabilityFeature) {
		t.Skip("Skipping test as durability not supported")
	}

	var doc testBeerDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not read test dataset: %v", err)
	}

	type CasResult interface {
		Cas() Cas
	}

	type tCase struct {
		name              string
		method            string
		args              []interface{}
		expectCas         bool
		expectedError     error
		expectKeyNotFound bool
	}

	testCases := []tCase{
		{
			name:   "insertDurabilityMajorityDoc",
			method: "Insert",
			args: []interface{}{"insertDurabilityMajorityDoc", doc, &InsertOptions{
				DurabilityLevel: DurabilityLevelMajority,
			}},
			expectCas:         true,
			expectedError:     nil,
			expectKeyNotFound: false,
		},
	}

	for _, tCase := range testCases {
		t.Run(tCase.name, func(te *testing.T) {
			args := make([]reflect.Value, len(tCase.args))
			for i := range tCase.args {
				args[i] = reflect.ValueOf(tCase.args[i])
			}

			retVals := reflect.ValueOf(globalCollection).MethodByName(tCase.method).Call(args)
			if len(retVals) != 2 {
				te.Fatalf("Method call should have returned 2 values but returned %d", len(retVals))
			}

			var retErr error
			if retVals[1].Interface() != nil {
				var ok bool
				retErr, ok = retVals[1].Interface().(error)
				if ok {
					if err != nil {
						te.Fatalf("Method call returned error: %v", err)
					}
				} else {
					te.Fatalf("Could not type assert second returned value to error")
				}
			}

			if retErr != tCase.expectedError {
				te.Fatalf("Expected error to be %v but was %v", tCase.expectedError, retErr)
			}

			if tCase.expectCas {
				if val, ok := retVals[0].Interface().(CasResult); ok {
					if val.Cas() == 0 {
						te.Fatalf("CAS value was 0")
					}
				} else {
					te.Fatalf("Could not assert result to CasResult type")
				}
			}

			_, err := globalCollection.GetAnyReplica(tCase.name, nil)
			if tCase.expectKeyNotFound {
				if !IsKeyNotFoundError(err) {
					t.Fatalf("Expected GetFromReplica to not find a key but got error %v", err)
				}
			} else {
				if err != nil {
					te.Fatalf("GetFromReplica failed, error was %v", err)
				}
			}
		})
	}
}

// In this test it is expected that the operation will timeout and ctx.Err() will be DeadlineExceeded.
func TestGetContextTimeout1(t *testing.T) {
	var doc testBreweryDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not load dataset: %v", err)
	}

	provider := &mockKvProvider{
		cas:                   gocbcore.Cas(0),
		datatype:              1,
		value:                 nil,
		opWait:                3000 * time.Millisecond,
		opCancellationSuccess: true,
	}
	col := testGetCollection(t, provider)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Millisecond)
	defer cancel()
	opts := GetOptions{Context: ctx, Timeout: 2000 * time.Millisecond}
	_, err = col.Get("getDocTimeout", &opts)
	if err == nil {
		t.Fatalf("Get succeeded, should have timedout")
	}

	if !IsTimeoutError(err) {
		t.Fatalf("Error should have been timeout error, was %s", reflect.TypeOf(err).Name())
	}

	if ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("Error should have been DeadlineExceeded error but was %v", ctx.Err())
	}
}

// In this test it is expected that the operation will timeout but ctx.Err() will be nil as it is the timeout value
// that is hit.
func TestGetContextTimeout2(t *testing.T) {
	var doc testBreweryDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not load dataset: %v", err)
	}

	provider := &mockKvProvider{
		cas:                   gocbcore.Cas(0),
		datatype:              1,
		value:                 nil,
		opWait:                2000 * time.Millisecond,
		opCancellationSuccess: true,
	}
	col := testGetCollection(t, provider)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	opts := GetOptions{Context: ctx, Timeout: 2 * time.Millisecond}
	_, err = col.Get("getDocTimeout", &opts)
	if err == nil {
		t.Fatalf("Insert succeeded, should have timedout")
	}

	if !IsTimeoutError(err) {
		t.Fatalf("Error should have been timeout error, was %s", reflect.TypeOf(err).Name())
	}

	if ctx.Err() != nil {
		t.Fatalf("Context error should have been nil")
	}
}

func TestGetErrorCollectionUnknown(t *testing.T) {
	var doc testBreweryDocument
	err := loadJSONTestDataset("beer_sample_single", &doc)
	if err != nil {
		t.Fatalf("Could not load dataset: %v", err)
	}

	provider := &mockKvProvider{
		err: &gocbcore.KvError{Code: gocbcore.StatusCollectionUnknown},
	}
	col := testGetCollection(t, provider)

	res, err := col.Get("getDocErrCollectionUnknown", nil)
	if err == nil {
		t.Fatalf("Get didn't error")
	}

	if res != nil {
		t.Fatalf("Result should have been nil")
	}

	if !IsCollectionNotFoundError(err) {
		t.Fatalf("Error should have been collection missing but was %v", err)
	}
}

// In this test it is expected that the operation will timeout and ctx.Err() will be DeadlineExceeded.
func TestInsertContextTimeout1(t *testing.T) {
	provider := &mockKvProvider{
		cas:                   gocbcore.Cas(0),
		datatype:              1,
		value:                 nil,
		opWait:                3000 * time.Millisecond,
		opCancellationSuccess: true,
	}
	col := testGetCollection(t, provider)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Millisecond)
	defer cancel()
	opts := InsertOptions{Context: ctx, Timeout: 2000 * time.Millisecond}
	_, err := col.Insert("insertDocTimeout", "test", &opts)
	if err == nil {
		t.Fatalf("Insert succeeded, should have timedout")
	}

	if !IsTimeoutError(err) {
		t.Fatalf("Error should have been timeout error, was %s", reflect.TypeOf(err).Name())
	}

	if ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("Error should have been DeadlineExceeded error but was %v", ctx.Err())
	}
}

// In this test it is expected that the operation will timeout but ctx.Err() will be nil as it is the timeout value
// that is hit.
func TestInsertContextTimeout2(t *testing.T) {
	provider := &mockKvProvider{
		cas:                   gocbcore.Cas(0),
		datatype:              1,
		value:                 nil,
		opWait:                2000 * time.Millisecond,
		opCancellationSuccess: true,
	}
	col := testGetCollection(t, provider)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	opts := InsertOptions{Context: ctx, Timeout: 2 * time.Millisecond}
	_, err := col.Insert("insertDocTimeout", "test", &opts)
	if err == nil {
		t.Fatalf("Insert succeeded, should have timedout")
	}

	if !IsTimeoutError(err) {
		t.Fatalf("Error should have been timeout error, was %s", reflect.TypeOf(err).Name())
	}

	if ctx.Err() != nil {
		t.Fatalf("Context error should have been nil")
	}
}

func TestCollectionContext(t *testing.T) {
	type args struct {
		ctx     context.Context
		timeout time.Duration
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	tests := []struct {
		name        string
		sb          stateBlock
		args        args
		wantBetween []time.Duration
	}{
		{
			name:        "No parameters should take cluster level timeout",
			sb:          stateBlock{KvTimeout: 20 * time.Second},
			args:        args{ctx: nil, timeout: 0},
			wantBetween: []time.Duration{19 * time.Second, 20 * time.Second},
		},
		{
			name:        "Timeout parameter only should be timeout",
			sb:          stateBlock{KvTimeout: 20 * time.Second},
			args:        args{ctx: nil, timeout: 30 * time.Second},
			wantBetween: []time.Duration{29 * time.Second, 30 * time.Second},
		},
		{
			name:        "Context parameter only should take cluster timeout",
			sb:          stateBlock{KvTimeout: 20 * time.Second},
			args:        args{ctx: ctx, timeout: 0},
			wantBetween: []time.Duration{19 * time.Second, 20 * time.Second},
		},
		{
			name:        "Context and timeout parameters, lower context should take context",
			sb:          stateBlock{KvTimeout: 20 * time.Second},
			args:        args{ctx: ctx, timeout: 30 * time.Second},
			wantBetween: []time.Duration{24 * time.Second, 25 * time.Second},
		},
		{
			name:        "Context and timeout parameters, lower timeout should take timeout",
			sb:          stateBlock{KvTimeout: 20 * time.Second},
			args:        args{ctx: ctx, timeout: 15 * time.Second},
			wantBetween: []time.Duration{14 * time.Second, 15 * time.Second},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Collection{
				sb: tt.sb,
			}
			ctx, cancel := c.context(tt.args.ctx, tt.args.timeout)
			d, ok := ctx.Deadline()
			if ok {
				timeout := d.Sub(time.Now())
				if timeout < tt.wantBetween[0] || timeout > tt.wantBetween[1] {
					t.Errorf(
						"Expected context to be between %f and %f but was %f",
						tt.wantBetween[0].Seconds(),
						tt.wantBetween[1].Seconds(),
						timeout.Seconds(),
					)
				}
			} else {
				t.Errorf("Expected context to have deadline, but didn't")
			}

			if cancel == nil {
				t.Errorf("Expected cancel func be not nil")
			}
		})
	}
}
