// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/otfabric/go-canopen/cdcf"
	"github.com/otfabric/go-canopen/cia402"
	"github.com/otfabric/go-canopen/node"
	"github.com/otfabric/go-canopen/od"
	"github.com/otfabric/go-canopen/program"
	"github.com/otfabric/go-canopen/store"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

var (
	schemaOnce sync.Once
	schemas    map[string]*jsonschema.Schema
	schemaErr  error
)

// schema returns a compiled schema of ../schemas: "result", "record" or
// "error".
func schema(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	schemaOnce.Do(func() {
		schemas = map[string]*jsonschema.Schema{}
		c := jsonschema.NewCompiler()
		for _, n := range []string{"result", "record", "error"} {
			s, err := c.Compile(filepath.Join("..", "schemas", n+"-v1.schema.json"))
			if err != nil {
				schemaErr = err
				return
			}
			schemas[n] = s
		}
	})
	if schemaErr != nil {
		t.Fatal(schemaErr)
	}
	return schemas[name]
}

// validate checks one JSON document against a schema.
func validate(t *testing.T, name, doc string) {
	t.Helper()
	v, err := jsonschema.UnmarshalJSON(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("not JSON: %v\n%s", err, doc)
	}
	if err := schema(t, name).Validate(v); err != nil {
		t.Fatalf("%s schema: %v\n%s", name, err, doc)
	}
}

// validateOutput checks a command's stdout: a result or error document,
// or JSON Lines records.
func validateOutput(t *testing.T, r result) {
	t.Helper()
	out := strings.TrimSpace(r.out)
	if out == "" {
		t.Fatalf("no output (exit %d): %s", r.code, r.errOut)
	}
	switch {
	case strings.HasPrefix(out, "{\n") && strings.Contains(out, `"error": {`):
		validate(t, "error", out)
	case strings.HasPrefix(out, "{\n"):
		validate(t, "result", out)
	default:
		for _, l := range strings.Split(out, "\n") {
			validate(t, "record", l)
		}
	}
}

// TestSchemasGolden validates every golden JSON file.
func TestSchemasGolden(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("testdata", "golden", "*.json*"))
	if len(files) == 0 {
		t.Fatal("no golden JSON files")
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		validateOutput(t, result{out: string(b)})
	}
}

// TestSchemasCommands runs every command with --format json against
// simulated nodes and validates what it writes.
func TestSchemasCommands(t *testing.T) {
	fresh(t)
	dir := t.TempDir()
	io := ioNode(t, 5)
	_ = io.Dictionary().SetUint(od.IndexProducerHeartbeat, 0, 10)
	drv := builtinNode(t, 3, true, node.WithDrive(cia402.DriveConfig{}, time.Millisecond))
	_ = drv
	j := func(args ...string) result {
		t.Helper()
		r := run(t, "", append(args, "--format", "json")...)
		validateOutput(t, r)
		return r
	}
	v := []string{"-i", "virtual"}
	cmds := [][]string{
		{"version"},
		{"sdo", "read", "5", "0x1018", "1"},
		{"sdo", "read", "5", "0x1018", "1", "--eds", "testdata/io401.eds"},
		{"sdo", "read", "5", "0x5FFF", "0"},
		{"sdo", "write", "5", "0x1017", "0", "10", "-t", "u16", "--verify"},
		{"sdo", "dump", "5", "--eds", "testdata/io401.eds", "--range", "1000-1018"},
		{"sdo", "dump", "3", "--eds", "testdata/io401.eds", "--range", "6000-6200"},
		{"nmt", "start", "5", "--wait"},
		{"scan", "--nodes", "1-8", "--timeout", "30ms"},
		{"sync", "--count", "2", "--period", "1ms"},
		{"time", "--at", "2026-10-29T12:00:00Z"},
		{"pdo", "show", "5", "--eds", "testdata/io401.eds"},
		{"pdo", "map", "3", "--tpdo", "3", "--map", "6041:00:16", "--type", "255", "--event-timer", "10"},
		{"nmt", "start", "all"},
		{"pdo", "watch", "3", "--tpdo", "3", "--count", "1", "--duration", "5s"},
		{"monitor", "--count", "5"},
		{"heartbeat", "watch", "--duration", "50ms", "--node", "5", "--supervise", "1s"},
		{"cia402", "state", "3"},
		{"cia402", "enable", "3", "--poll", "1ms"},
		{"cia402", "state", "5"},
		{"dcf", "apply", "5", "testdata/node37.dcf", "--dry-run"},
		{"dcf", "export", "5", "--eds", "testdata/io401.eds", "-o", filepath.Join(dir, "x.dcf")},
		{"eds", "validate", "testdata/broken.dcf"},
		{"eds", "show", "testdata/drive402.eds"},
		{"eds", "convert", "testdata/io401.eds", "--node-id", "5", "-o", filepath.Join(dir, "y.dcf")},
		{"eds", "generate", "testdata/io401.eds", "--package", "x", "-o", filepath.Join(dir, "x.go")},
		{"eds", "validate", "testdata/missing.eds"},
		{"nmt", "start", "all", "-i", "slcan:/dev/null"},
		{"bogus"},
	}
	for _, c := range cmds {
		j(append(c, v...)...)
	}
	// Streams that need traffic.
	stop := raiseUntil(io, 0x8130)
	j("emcy", "watch", "--count", "2", "-i", "virtual")
	stop()
	path := filepath.Join(dir, "t.log")
	_ = os.WriteFile(path, []byte(trace), 0o644)
	j("decode", path)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan result, 1)
	go func() { done <- runCtx(ctx, "simulate", "-i", "virtual", "--node-id", "9", "--format", "json") }()
	time.Sleep(50 * time.Millisecond)
	want(t, run(t, "", "nmt", "start", "9", "-i", "virtual", "--wait"), 0)
	cancel()
	r := <-done
	validateOutput(t, r)
}

// TestGoldenJSON pins the JSON of deterministic results.
func TestGoldenJSON(t *testing.T) {
	fresh(t)
	ioNode(t, 5)
	golden(t, "sdo-read.json", run(t, "", "sdo", "read", "5", "0x1018", "2", "--eds", "testdata/io401.eds", "-i", "virtual", "--format", "json").out)
	golden(t, "sdo-abort.json", run(t, "", "sdo", "read", "5", "0x5FFF", "0", "-i", "virtual", "--format", "json").out)
	golden(t, "scan.json", run(t, "", "scan", "--nodes", "4-6", "-i", "virtual", "--format", "json").out)
	golden(t, "dcf-apply-dry.json", run(t, "", "dcf", "apply", "37", "testdata/node37.dcf", "--dry-run", "--format", "json").out)
	golden(t, "needs-yes.json", run(t, "", "sdo", "write", "5", "0x1017", "0", "1", "-t", "u16", "-i", "slcan:/dev/null", "--format", "json").out)
}

// TestSchemasCiA302 runs the LSS, MPDO, CiA 302 and CANopen FD commands
// with --format json and validates what they write.
func TestSchemasCiA302(t *testing.T) {
	fresh(t)
	dir := t.TempDir()
	lssNode(t, 0, 0x2001)
	p, _ := mpdoNodes(t)
	_ = p
	d, _ := builtinDictionary(8, false)
	_ = store.AddObjects(d)
	simNode(t, d, node.WithStore(store.NewMemory()), node.WithProgram(program.Config{Handler: &memProgram{}}))
	fw := filepath.Join(dir, "fw.bin")
	_ = os.WriteFile(fw, []byte("firmware"), 0o644)
	cfg := writeConcise(t, cdcf.Entry{Index: simText, Data: []byte("x")})
	j := func(args ...string) {
		t.Helper()
		validateOutput(t, run(t, "", append(args, "--format", "json")...))
	}
	v := []string{"-i", "virtual"}
	a := lssAddr(0x2001)
	for _, c := range [][]string{
		{"lss", "switch", "waiting"},
		{"lss", "identify"},
		{"lss", "identify", "--address", a},
		{"lss", "fastscan", "--window", "10ms", "--vendor", "0xF7A", "--product", "0xC0C7"},
		{"lss", "inquire", "--address", a},
		{"lss", "select", a},
		{"lss", "inquire"},
		{"lss", "configure", "--node-id", "none", "--bit-rate", "auto"},
		{"lss", "activate", "--delay", "0s"},
		{"lss", "switch", "waiting"},
		{"lss", "assign-all", "--first", "20", "--window", "10ms", "--vendor", "0xF7A", "--product", "0xC0C7"},
		{"lss", "select", "1:2:3:4", "--timeout", "10ms", "--retries", "0"},
		{"mpdo", "configure", "5", "--tpdo", "2", "--mode", "sam", "--cob-id", "285"},
		{"mpdo", "scanner", "5", "--entry", "2001:00"},
		{"mpdo", "dispatcher", "6", "--entry", "5:2001:00=2001:00"},
		{"mpdo", "watch", "--cob-id", "285", "--duration", "10ms"},
		{"cdcf", "convert", "testdata/node37.dcf", "-o", filepath.Join(dir, "x.cdcf")},
		{"cdcf", "show", cfg},
		{"cdcf", "apply", "8", cfg},
		{"cdcf", "apply", "8", cfg, "--dry-run"},
		{"store", "8"},
		{"restore", "8"},
		{"program", "download", "8", fw, "--poll", "5ms"},
		{"program", "control", "8", "stop"},
		{"program", "status", "8"},
		{"boot", "--slave", "8=" + cfg, "--slave", "9", "--optional", "9", "--no-start"},
		{"simulate", "--node-id", "30", "--duration", "10ms", "--fd", "--program"},
	} {
		j(append(c, v...)...)
	}
}
