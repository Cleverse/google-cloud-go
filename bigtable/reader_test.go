/*
Copyright 2016 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package bigtable

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"cloud.google.com/go/internal/testutil"
	btspb "google.golang.org/genproto/googleapis/bigtable/v2"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Indicates that a field in the proto should be omitted, rather than included
// as a wrapped empty string.
const nilStr = "<>"

func TestSingleCell(t *testing.T) {
	crs := []*chunkReader{
		newChunkReader(true),
		newChunkReader(false),
	}

	for i, cr := range crs {
		t.Run(fmt.Sprint("#", i), func(t *testing.T) {
			// All in one cell
			row, err := cr.Process(cc("rk", "fm", "col", 1, "value", 0, true, []string{}))
			if err != nil {
				t.Fatalf("Processing chunk: %v", err)
			}
			if row == nil {
				t.Fatalf("Missing row")
			}
			if len(row["fm"]) != 1 {
				t.Fatalf("Family name length mismatch %d, %d", 1, len(row["fm"]))
			}
			want := []ReadItem{ri("rk", "fm", "col", 1, "value", []string{})}
			if !testutil.Equal(row["fm"], want) {
				t.Fatalf("Incorrect ReadItem: got: %v\nwant: %v\n", row["fm"], want)
			}
			if err := cr.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
		})
	}
}

func TestMultipleCells(t *testing.T) {
	crs := []*chunkReader{
		newChunkReader(true),
		newChunkReader(false),
	}

	for i, cr := range crs {
		t.Run(fmt.Sprint("#", i), func(t *testing.T) {
			mustProcess(t, cr, cc("rs", "fm1", "col1", 0, "val1", 0, false, []string{}))
			mustProcess(t, cr, cc("rs", "fm1", "col1", 1, "val2", 0, false, []string{}))
			mustProcess(t, cr, cc("rs", "fm1", "col2", 0, "val3", 0, false, []string{}))
			mustProcess(t, cr, cc("rs", "fm2", "col1", 0, "val4", 0, false, []string{}))
			row, err := cr.Process(cc("rs", "fm2", "col2", 1, "extralongval5", 0, true, []string{}))
			if err != nil {
				t.Fatalf("Processing chunk: %v", err)
			}
			if row == nil {
				t.Fatalf("Missing row")
			}

			want := []ReadItem{
				ri("rs", "fm1", "col1", 0, "val1", []string{}),
				ri("rs", "fm1", "col1", 1, "val2", []string{}),
				ri("rs", "fm1", "col2", 0, "val3", []string{}),
			}
			if !testutil.Equal(row["fm1"], want) {
				t.Fatalf("Incorrect ReadItem: got: %v\nwant: %v\n", row["fm1"], want)
			}
			want = []ReadItem{
				ri("rs", "fm2", "col1", 0, "val4", []string{}),
				ri("rs", "fm2", "col2", 1, "extralongval5", []string{}),
			}
			if !testutil.Equal(row["fm2"], want) {
				t.Fatalf("Incorrect ReadItem: got: %v\nwant: %v\n", row["fm2"], want)
			}
			if err := cr.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
		})
	}
}

func TestSplitCells(t *testing.T) {
	crs := []*chunkReader{
		newChunkReader(true),
		newChunkReader(false),
	}

	for i, cr := range crs {
		t.Run(fmt.Sprint("#", i), func(t *testing.T) {
			mustProcess(t, cr, cc("rs", "fm1", "col1", 0, "hello ", 11, false, []string{}))
			mustProcess(t, cr, ccData("world", 0, false))
			row, err := cr.Process(cc("rs", "fm1", "col2", 0, "val2", 0, true, []string{}))
			if err != nil {
				t.Fatalf("Processing chunk: %v", err)
			}
			if row == nil {
				t.Fatalf("Missing row")
			}

			want := []ReadItem{
				ri("rs", "fm1", "col1", 0, "hello world", []string{}),
				ri("rs", "fm1", "col2", 0, "val2", []string{}),
			}
			if !testutil.Equal(row["fm1"], want) {
				t.Fatalf("Incorrect ReadItem: got: %v\nwant: %v\n", row["fm1"], want)
			}
			if err := cr.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
		})
	}
}

func TestMultipleRows(t *testing.T) {
	reversedScan := false
	cr := newChunkReader(reversedScan)

	row, err := cr.Process(cc("rs1", "fm1", "col1", 1, "val1", 0, true, []string{}))
	if err != nil {
		t.Fatalf("Processing chunk: %v", err)
	}
	want := []ReadItem{ri("rs1", "fm1", "col1", 1, "val1", []string{})}
	if !testutil.Equal(row["fm1"], want) {
		t.Fatalf("Incorrect ReadItem: got: %v\nwant: %v\n", row["fm1"], want)
	}

	row, err = cr.Process(cc("rs2", "fm2", "col2", 2, "val2", 0, true, []string{}))
	if err != nil {
		t.Fatalf("Processing chunk: %v", err)
	}
	want = []ReadItem{ri("rs2", "fm2", "col2", 2, "val2", []string{})}
	if !testutil.Equal(row["fm2"], want) {
		t.Fatalf("Incorrect ReadItem: got: %v\nwant: %v\n", row["fm2"], want)
	}

	if err := cr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestReverseScanMultipleRows(t *testing.T) {
	reversedScan := true
	cr := newChunkReader(reversedScan)

	row, err := cr.Process(cc("rs3", "fm3", "col3", 3, "val3", 0, true, []string{}))
	if err != nil {
		t.Fatalf("Processing chunk: %v", err)
	}
	want := []ReadItem{ri("rs3", "fm3", "col3", 3, "val3", []string{})}
	if !testutil.Equal(row["fm3"], want) {
		t.Fatalf("Incorrect ReadItem: got: %v\nwant: %v\n", row["fm2"], want)
	}

	row, err = cr.Process(cc("rs2", "fm2", "col2", 2, "val2", 0, true, []string{}))
	if err != nil {
		t.Fatalf("Processing chunk: %v", err)
	}
	want = []ReadItem{ri("rs2", "fm2", "col2", 2, "val2", []string{})}
	if !testutil.Equal(row["fm2"], want) {
		t.Fatalf("Incorrect ReadItem: got: %v\nwant: %v\n", row["fm2"], want)
	}

	row, err = cr.Process(cc("rs1", "fm1", "col1", 1, "val1", 0, true, []string{}))
	if err != nil {
		t.Fatalf("Processing chunk: %v", err)
	}
	want = []ReadItem{ri("rs1", "fm1", "col1", 1, "val1", []string{})}
	if !testutil.Equal(row["fm1"], want) {
		t.Fatalf("Incorrect ReadItem: got: %v\nwant: %v\n", row["fm1"], want)
	}

	if err := cr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestBlankQualifier(t *testing.T) {
	reversedScan := false
	cr := newChunkReader(reversedScan)

	row, err := cr.Process(cc("rs1", "fm1", "", 1, "val1", 0, true, []string{}))
	if err != nil {
		t.Fatalf("Processing chunk: %v", err)
	}
	want := []ReadItem{ri("rs1", "fm1", "", 1, "val1", []string{})}
	if !testutil.Equal(row["fm1"], want) {
		t.Fatalf("Incorrect ReadItem: got: %v\nwant: %v\n", row["fm1"], want)
	}

	row, err = cr.Process(cc("rs2", "fm2", "col2", 2, "val2", 0, true, []string{}))
	if err != nil {
		t.Fatalf("Processing chunk: %v", err)
	}
	want = []ReadItem{ri("rs2", "fm2", "col2", 2, "val2", []string{})}
	if !testutil.Equal(row["fm2"], want) {
		t.Fatalf("Incorrect ReadItem: got: %v\nwant: %v\n", row["fm2"], want)
	}

	if err := cr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestReverseScanBlankQualifier(t *testing.T) {
	reversedScan := true
	cr := newChunkReader(reversedScan)

	row, err := cr.Process(cc("rs2", "fm2", "col2", 2, "val2", 0, true, []string{}))
	if err != nil {
		t.Fatalf("Processing chunk: %v", err)
	}
	want := []ReadItem{ri("rs2", "fm2", "col2", 2, "val2", []string{})}
	if !testutil.Equal(row["fm2"], want) {
		t.Fatalf("Incorrect ReadItem: got: %v\nwant: %v\n", row["fm2"], want)
	}

	row, err = cr.Process(cc("rs1", "fm1", "", 1, "val1", 0, true, []string{}))
	if err != nil {
		t.Fatalf("Processing chunk: %v", err)
	}
	want = []ReadItem{ri("rs1", "fm1", "", 1, "val1", []string{})}
	if !testutil.Equal(row["fm1"], want) {
		t.Fatalf("Incorrect ReadItem: got: %v\nwant: %v\n", row["fm1"], want)
	}

	if err := cr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestLabels(t *testing.T) {
	reversedScan := false
	cr := newChunkReader(reversedScan)

	mustProcess(t, cr, cc("rs1", "fm1", "col1", 0, "hello ", 11, false, []string{"test-label"}))
	row := mustProcess(t, cr, ccData("world", 0, true))
	want := []ReadItem{
		ri("rs1", "fm1", "col1", 0, "hello world", []string{"test-label"}),
	}
	if !testutil.Equal(row["fm1"], want) {
		t.Fatalf("Incorrect ReadItem: got: %v\nwant: %v\n", row["fm1"], want)
	}

	row, err := cr.Process(cc("rs2", "fm1", "", 1, "val1", 0, true, []string{"test-label2"}))
	if err != nil {
		t.Fatalf("Processing chunk: %v", err)
	}
	want = []ReadItem{ri("rs2", "fm1", "", 1, "val1", []string{"test-label2"})}
	if !testutil.Equal(row["fm1"], want) {
		t.Fatalf("Incorrect ReadItem: got: %v\nwant: %v\n", row["fm1"], want)
	}

	if err := cr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestReverseScanLabels(t *testing.T) {
	reversedScan := true
	cr := newChunkReader(reversedScan)

	mustProcess(t, cr, cc("rs2", "fm1", "col1", 0, "hello ", 11, false, []string{"test-label2"}))
	row := mustProcess(t, cr, ccData("world", 0, true))
	want := []ReadItem{
		ri("rs2", "fm1", "col1", 0, "hello world", []string{"test-label2"}),
	}
	if !testutil.Equal(row["fm1"], want) {
		t.Fatalf("Incorrect ReadItem: got: %v\nwant: %v\n", row["fm1"], want)
	}

	row, err := cr.Process(cc("rs1", "fm1", "", 1, "val1", 0, true, []string{"test-label"}))
	if err != nil {
		t.Fatalf("Processing chunk: %v", err)
	}
	want = []ReadItem{ri("rs1", "fm1", "", 1, "val1", []string{"test-label"})}
	if !testutil.Equal(row["fm1"], want) {
		t.Fatalf("Incorrect ReadItem: got: %v\nwant: %v\n", row["fm1"], want)
	}

	if err := cr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestReset(t *testing.T) {
	crs := []*chunkReader{
		newChunkReader(true),
		newChunkReader(false),
	}

	for i, cr := range crs {
		t.Run(fmt.Sprint("#", i), func(t *testing.T) {
			mustProcess(t, cr, cc("rs", "fm1", "col1", 0, "val1", 0, false, []string{}))
			mustProcess(t, cr, cc("rs", "fm1", "col1", 1, "val2", 0, false, []string{}))
			mustProcess(t, cr, cc("rs", "fm1", "col2", 0, "val3", 0, false, []string{}))
			mustProcess(t, cr, ccReset())
			row := mustProcess(t, cr, cc("rs1", "fm1", "col1", 1, "val1", 0, true, []string{}))
			want := []ReadItem{ri("rs1", "fm1", "col1", 1, "val1", []string{})}
			if !testutil.Equal(row["fm1"], want) {
				t.Fatalf("Reset: got: %v\nwant: %v\n", row["fm1"], want)
			}
			if err := cr.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
		})
	}
}

func TestNewFamEmptyQualifier(t *testing.T) {
	crs := []*chunkReader{
		newChunkReader(true),
		newChunkReader(false),
	}

	for i, cr := range crs {
		t.Run(fmt.Sprint("#", i), func(t *testing.T) {
			mustProcess(t, cr, cc("rs", "fm1", "col1", 0, "val1", 0, false, []string{}))
			_, err := cr.Process(cc(nilStr, "fm2", nilStr, 0, "val2", 0, true, []string{}))
			if err == nil {
				t.Fatalf("Expected error on second chunk with no qualifier set")
			}
		})
	}
}

func mustProcess(t *testing.T, cr *chunkReader, cc *btspb.ReadRowsResponse_CellChunk) Row {
	row, err := cr.Process(cc)
	if err != nil {
		t.Fatal(err)
	}
	return row
}

// The read rows acceptance test reads a json file specifying a number of tests,
// each consisting of one or more cell chunk text protos and one or more resulting
// cells or errors.
type AcceptanceTest struct {
	Tests []TestCase `json:"tests"`
}

type TestCase struct {
	Name    string       `json:"name"`
	Chunks  []string     `json:"chunks"`
	Results []TestResult `json:"results"`
}

type TestResult struct {
	RK    string `json:"rk"`
	FM    string `json:"fm"`
	Qual  string `json:"qual"`
	TS    int64  `json:"ts"`
	Value string `json:"value"`
	Error bool   `json:"error"` // If true, expect an error. Ignore any other field.
}

func TestAcceptance(t *testing.T) {
	testJSON, err := ioutil.ReadFile("./testdata/read-rows-acceptance-test.json")
	if err != nil {
		t.Fatalf("could not open acceptance test file %v", err)
	}

	var accTest AcceptanceTest
	err = json.Unmarshal(testJSON, &accTest)
	if err != nil {
		t.Fatalf("could not parse acceptance test file: %v", err)
	}

	for _, test := range accTest.Tests {
		runTestCase(t, test)
	}
}

func runTestCase(t *testing.T, test TestCase) {
	// Increment an index into the result array as we get results
	reversedScan := false
	cr := newChunkReader(reversedScan)
	var results []TestResult
	var seenErr bool
	for _, chunkText := range test.Chunks {
		// Parse and pass each cell chunk to the ChunkReader
		cc := &btspb.ReadRowsResponse_CellChunk{}
		err := prototext.Unmarshal([]byte(chunkText), cc)
		if err != nil {
			t.Errorf("[%s] failed to unmarshal text proto: %s\n%s", test.Name, chunkText, err)
			return
		}
		row, err := cr.Process(cc)
		if err != nil {
			results = append(results, TestResult{Error: true})
			seenErr = true
			break
		} else {
			// Turn the Row into TestResults
			for fm, ris := range row {
				for _, ri := range ris {
					tr := TestResult{
						RK:    ri.Row,
						FM:    fm,
						Qual:  strings.Split(ri.Column, ":")[1],
						TS:    int64(ri.Timestamp),
						Value: string(ri.Value),
					}
					results = append(results, tr)
				}
			}
		}
	}

	// Only Close if we don't have an error yet, otherwise Close: is expected.
	if !seenErr {
		err := cr.Close()
		if err != nil {
			results = append(results, TestResult{Error: true})
		}
	}

	got := toSet(results)
	want := toSet(test.Results)
	if !testutil.Equal(got, want) {
		t.Fatalf("[%s]: got: %v\nwant: %v\n", test.Name, got, want)
	}
}

func toSet(res []TestResult) map[TestResult]bool {
	set := make(map[TestResult]bool)
	for _, tr := range res {
		set[tr] = true
	}
	return set
}

// ri returns a ReadItem for the given components
func ri(rk string, fm string, qual string, ts int64, val string, labels []string) ReadItem {
	return ReadItem{Row: rk, Column: fmt.Sprintf("%s:%s", fm, qual), Value: []byte(val), Timestamp: Timestamp(ts), Labels: labels}
}

// cc returns a CellChunk proto
func cc(rk string, fm string, qual string, ts int64, val string, size int32, commit bool, labels []string) *btspb.ReadRowsResponse_CellChunk {
	// The components of the cell key are wrapped and can be null or empty
	var rkWrapper []byte
	if rk == nilStr {
		rkWrapper = nil
	} else {
		rkWrapper = []byte(rk)
	}

	var fmWrapper *wrapperspb.StringValue
	if fm != nilStr {
		fmWrapper = &wrapperspb.StringValue{Value: fm}
	} else {
		fmWrapper = nil
	}

	var qualWrapper *wrapperspb.BytesValue
	if qual != nilStr {
		qualWrapper = &wrapperspb.BytesValue{Value: []byte(qual)}
	} else {
		qualWrapper = nil
	}

	return &btspb.ReadRowsResponse_CellChunk{
		RowKey:          rkWrapper,
		FamilyName:      fmWrapper,
		Qualifier:       qualWrapper,
		TimestampMicros: ts,
		Value:           []byte(val),
		ValueSize:       size,
		RowStatus:       &btspb.ReadRowsResponse_CellChunk_CommitRow{CommitRow: commit},
		Labels:          labels,
	}
}

// ccData returns a CellChunk with only a value and size
func ccData(val string, size int32, commit bool) *btspb.ReadRowsResponse_CellChunk {
	return cc(nilStr, nilStr, nilStr, 0, val, size, commit, []string{})
}

// ccReset returns a CellChunk with RestRow set to true
func ccReset() *btspb.ReadRowsResponse_CellChunk {
	return &btspb.ReadRowsResponse_CellChunk{
		RowStatus: &btspb.ReadRowsResponse_CellChunk_ResetRow{ResetRow: true},
	}
}
