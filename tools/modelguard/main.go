package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
)

// FieldInfo stores details about a struct field
type FieldInfo struct {
	Name    string // Go field name
	Type    string // Type name (simple string representation)
	JSONKey string // JSON key from tag
}

// StructInfo stores fields of a struct
type StructInfo struct {
	Name   string
	Fields map[string]FieldInfo // Keyed by JSON Key
}

func main() {
	serverPath := flag.String("server", "server/internal/domain/models.go", "Path to server models")
	clientPath := flag.String("client", "client/internal/domain/models.go", "Path to client models")
	flag.Parse()

	// 1. Parse Files
	serverStructs, err := parseFile(*serverPath)
	if err != nil {
		fmt.Printf("Error parsing server file: %v\n", err)
		os.Exit(1)
	}

	clientStructs, err := parseFile(*clientPath)
	if err != nil {
		fmt.Printf("Error parsing client file: %v\n", err)
		os.Exit(1)
	}

	// 2. Compare Shared Structs
	structsToCheck := []string{
		"Player", "Island", "Fleet", "Ship", "Building", "ResourceNode",
		"PveTarget", "CombatResult", "EngagementResult", "Captain",
	}

	hasError := false
	hasWarning := false

	for _, sName := range structsToCheck {
		sStruct, sOk := serverStructs[sName]
		cStruct, cOk := clientStructs[sName]

		if !sOk && !cOk {
			continue // Skip if neither has it
		}
		if !sOk {
			fmt.Printf("WARNING: Struct %s exists in Client but not Server\n", sName)
			continue
		}
		if !cOk {
			fmt.Printf("WARNING: Struct %s exists in Server but not Client\n", sName)
			continue
		}

		// Check Drift
		missing, typeMismatch := compareStructs(sStruct, cStruct)

		if len(missing) > 0 {
			hasError = true
			fmt.Printf("\n[ERROR] Struct %s Drift Detected:\n", sName)
			for _, m := range missing {
				fmt.Printf("  - %s\n", m)
			}
		}

		if len(typeMismatch) > 0 {
			// Type mismatch is WARNING for now (Exit 0) unless strictly required
			hasWarning = true // Just flag it, printing is enough
			fmt.Printf("\n[WARNING] Struct %s Type Mismatches:\n", sName)
			for _, m := range typeMismatch {
				fmt.Printf("  ~ %s\n", m)
			}
		}
	}

	if hasError {
		fmt.Println("\n❌ DRIFT DETECTED: Critical JSON mismatches found.")
		os.Exit(1)
	}

	if hasWarning {
		fmt.Println("\n⚠️  WARNINGS: Type mismatches found (non-blocking).")
	} else {
		fmt.Println("\n✅ OK: Models are synchronized.")
	}
}

func parseFile(path string) (map[string]StructInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	structs := make(map[string]StructInfo)

	ast.Inspect(node, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}

		info := StructInfo{
			Name:   ts.Name.Name,
			Fields: make(map[string]FieldInfo),
		}

		for _, field := range st.Fields.List {
			if len(field.Names) == 0 {
				continue // Embedded fields or unnamed, verify logic
			}
			fieldName := field.Names[0].Name
			typeName := exprToString(field.Type) // Simplified type extraction
			tagValue := ""
			if field.Tag != nil {
				tagValue = reflect.StructTag(strings.Trim(field.Tag.Value, "`")).Get("json")
			}

			// JSON Tag Parsing Rule
			// 1. "-" => Ignore
			// 2. "" => Go field name (if exported, but usually we require tags), strictly speaking default is fieldName
			// 3. "name" => Key is "name"
			// 4. "name,omitempty" => Key is "name"
			// 5. ",omitempty" => Key is fieldName

			if tagValue == "-" {
				continue
			}

			parts := strings.Split(tagValue, ",")
			jsonKey := ""
			if len(parts) > 0 {
				jsonKey = parts[0]
			}

			if jsonKey == "" {
				// Fallback to Field Name (standard Go JSON behavior)
				jsonKey = fieldName
			}

			info.Fields[jsonKey] = FieldInfo{
				Name:    fieldName,
				Type:    typeName,
				JSONKey: jsonKey,
			}
		}

		structs[info.Name] = info
		return false
	})

	return structs, nil
}

func compareStructs(server, client StructInfo) (missing []string, typeMismatch []string) {
	// Check Server fields missing in Client
	for key, sField := range server.Fields {
		cField, exists := client.Fields[key]
		if !exists {
			missing = append(missing, fmt.Sprintf("Missing on Client: JSON key '%s' (Server: %s %s)", key, sField.Name, sField.Type))
		} else {
			// Check Type Mismatch
			// Normalizing types (e.g. *time.Time vs time.Time logic, or uuid.UUID vs string)
			// For now, strict string comparison but loose on pointers if needed?
			// Client often uses `string` where Server uses `uuid.UUID` or `time.Time`
			// This is a "Type Mismatch Warning" as requested.
			sType := normalizeType(sField.Type)
			cType := normalizeType(cField.Type)
			if sType != cType {
				typeMismatch = append(typeMismatch, fmt.Sprintf("Key '%s': Server %s vs Client %s", key, sField.Type, cField.Type))
			}
		}
	}

	// Check Client fields missing in Server
	for key, cField := range client.Fields {
		_, exists := server.Fields[key]
		if !exists {
			// Client has a field that Server DOES NOT send?
			// This assumes strict one-to-one mapping.
			// Ideally Server is source of truth. If Client expects 'foo' and Server doesn't have it, it's a bug?
			// Or maybe Client-only state.
			// Usually models in domain are DTOs.
			// Let's flag it.
			missing = append(missing, fmt.Sprintf("Missing on Server: JSON key '%s' (Client: %s %s)", key, cField.Name, cField.Type))
		}
	}

	return
}

func normalizeType(t string) string {
	// Remove pointer stars for looser comparison?
	// Or keep strict?
	// User Requirement: "server uuid.UUID vs client string" is a mismatch example.
	// So we keep raw strings.
	return strings.TrimPrefix(t, "*")
}

func exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprToString(t.X)
	case *ast.SelectorExpr:
		return exprToString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprToString(t.Elt)
	case *ast.MapType:
		return "map[" + exprToString(t.Key) + "]" + exprToString(t.Value)
	default:
		return fmt.Sprintf("%T", t)
	}
}
