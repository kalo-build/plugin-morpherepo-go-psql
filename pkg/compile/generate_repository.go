package compile

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/kalo-build/morphe-go/pkg/yaml"
	"github.com/kalo-build/plugin-morpherepo-go-psql/pkg/compile/cfg"
	"github.com/kalo-build/plugin-morpherepo-go/pkg/repo"
)

// columnInfo describes a database column and its mapping to a Go model field.
type columnInfo struct {
	columnName string // SQL column name (snake_case)
	fieldName  string // Go struct field name (PascalCase)
	isPointer  bool   // whether the Go field is a pointer (e.g., *string for FK)
	morpheType string // Morphe field type (e.g., "UUID", "String"); empty for FK columns
}

// GenerateRepository generates a Go file with a concrete pgxpool-backed repository
// implementation for the given repo spec and morphe model.
func GenerateRepository(spec repo.RepoSpec, model yaml.Model, allModels map[string]yaml.Model, config cfg.CompileConfig) string {
	var b strings.Builder

	repoPkg := config.RepoPackageName()
	modelsPkg := config.ModelsPackageName()
	table := tableName(spec.Model)
	columns := extractColumns(model)

	// Package declaration
	b.WriteString(fmt.Sprintf("package %s\n\n", repoPkg))

	// Imports — always include fmt, strings (used by Query), and pgx (used by QueryOne)
	b.WriteString("import (\n")
	b.WriteString("\t\"context\"\n")
	b.WriteString("\t\"fmt\"\n")
	b.WriteString("\t\"strings\"\n")
	b.WriteString("\n")
	b.WriteString("\t\"github.com/jackc/pgx/v5\"\n")
	b.WriteString("\t\"github.com/jackc/pgx/v5/pgxpool\"\n")
	b.WriteString(fmt.Sprintf("\t\"%s\"\n", config.ModelsImportPath()))
	b.WriteString(")\n\n")

	// Struct definition
	structName := spec.Model + "Repository"
	b.WriteString(fmt.Sprintf("// %s implements the %s interface using PostgreSQL.\n", structName, spec.Name))
	b.WriteString(fmt.Sprintf("type %s struct {\n", structName))
	b.WriteString("\tpool *pgxpool.Pool\n")
	b.WriteString("}\n\n")

	// Constructor
	b.WriteString(fmt.Sprintf("// New%s creates a new %s.\n", structName, structName))
	b.WriteString(fmt.Sprintf("func New%s(pool *pgxpool.Pool) *%s {\n", structName, structName))
	b.WriteString(fmt.Sprintf("\treturn &%s{pool: pool}\n", structName))
	b.WriteString("}\n\n")

	hasFilters := len(spec.Filters) > 0

	// Generate CRUD operations
	if spec.Operations.List {
		writeGetAll(&b, spec, model, columns, table, modelsPkg, hasFilters)
	}

	if spec.Operations.Get {
		writeGetByIdentifiers(&b, spec, columns, table, modelsPkg)
	}

	if spec.Operations.Create {
		writeCreate(&b, spec, columns, table, modelsPkg)
	}

	if spec.Operations.Update {
		writeUpdate(&b, spec, model, columns, table, modelsPkg)
	}

	if spec.Operations.Delete {
		writeDelete(&b, spec, table)
	}

	// Generate Query/QueryOne (always generated)
	writeQuery(&b, spec, model, columns, table, modelsPkg, allModels)
	writeQueryOne(&b, spec, modelsPkg)

	return b.String()
}

// extractColumns derives the full column list from a morphe model.
func extractColumns(model yaml.Model) []columnInfo {
	var columns []columnInfo

	// Direct fields (sorted by name)
	fieldNames := sortedMapKeys(model.Fields)
	for _, name := range fieldNames {
		columns = append(columns, columnInfo{
			columnName: toSnakeCase(name),
			fieldName:  name,
			isPointer:  false,
			morpheType: string(model.Fields[name].Type),
		})
	}

	// FK fields from relations (sorted by relation name)
	relNames := sortedMapKeys(model.Related)
	for _, relName := range relNames {
		rel := model.Related[relName]
		switch rel.Type {
		case "ForOne":
			columns = append(columns, columnInfo{
				columnName: toSnakeCase(relName) + "_id",
				fieldName:  relName + "ID",
				isPointer:  true,
				morpheType: "", // FK pointer field
			})
		case "ForOnePoly":
			through := relName
			if rel.Through != "" {
				through = rel.Through
			}
			columns = append(columns, columnInfo{
				columnName: toSnakeCase(through) + "_id",
				fieldName:  through + "ID",
				isPointer:  false,
				morpheType: "", // FK field
			})
			columns = append(columns, columnInfo{
				columnName: toSnakeCase(through) + "_type",
				fieldName:  through + "Type",
				isPointer:  false,
				morpheType: "", // FK type discriminator
			})
		}
	}

	return columns
}

// fkColumnForRelation returns the database column name for a filter's relation.
func fkColumnForRelation(model yaml.Model, relationName string) string {
	rel, ok := model.Related[relationName]
	if !ok {
		return toSnakeCase(relationName) + "_id"
	}
	switch rel.Type {
	case "ForOnePoly":
		through := relationName
		if rel.Through != "" {
			through = rel.Through
		}
		return toSnakeCase(through) + "_id"
	default:
		return toSnakeCase(relationName) + "_id"
	}
}

// selectColumns builds the comma-separated column list for SELECT.
func selectColumns(columns []columnInfo, alias string) string {
	parts := make([]string, len(columns))
	for i, c := range columns {
		if alias != "" {
			parts[i] = alias + "." + c.columnName
		} else {
			parts[i] = c.columnName
		}
	}
	return strings.Join(parts, ", ")
}

// scanArgs builds the scan arguments like "&m.ID, &m.Code, ...".
func scanArgs(columns []columnInfo) string {
	parts := make([]string, len(columns))
	for i, c := range columns {
		parts[i] = "&m." + c.fieldName
	}
	return strings.Join(parts, ", ")
}

// writeGetAll generates the GetAll method with optional filters.
func writeGetAll(b *strings.Builder, spec repo.RepoSpec, model yaml.Model, columns []columnInfo, table string, modelsPkg string, hasFilters bool) {
	// Build method signature
	params := []string{"ctx context.Context"}
	for _, f := range spec.Filters {
		params = append(params, fmt.Sprintf("%s *string", f.Name))
	}
	paramStr := strings.Join(params, ", ")

	b.WriteString(fmt.Sprintf("// GetAll retrieves all %s records with optional filters.\n", spec.Model))
	b.WriteString(fmt.Sprintf("func (r *%sRepository) GetAll(%s) ([]%s.%s, error) {\n", spec.Model, paramStr, modelsPkg, spec.Model))

	cols := selectColumns(columns, "")
	b.WriteString(fmt.Sprintf("\tquery := `SELECT %s FROM %s`\n", cols, table))

	if hasFilters {
		b.WriteString("\n")
		b.WriteString("\tvar conditions []string\n")
		b.WriteString("\tvar args []interface{}\n")
		b.WriteString("\tparamIdx := 1\n\n")

		for _, f := range spec.Filters {
			fkCol := fkColumnForRelation(model, f.Relation)
			b.WriteString(fmt.Sprintf("\tif %s != nil {\n", f.Name))
			b.WriteString(fmt.Sprintf("\t\tconditions = append(conditions, fmt.Sprintf(\"%s = $%%d\", paramIdx))\n", fkCol))
			b.WriteString(fmt.Sprintf("\t\targs = append(args, *%s)\n", f.Name))
			b.WriteString("\t\tparamIdx++\n")
			b.WriteString("\t}\n\n")
		}

		b.WriteString("\tif len(conditions) > 0 {\n")
		b.WriteString("\t\tquery += \" WHERE \" + strings.Join(conditions, \" AND \")\n")
		b.WriteString("\t}\n\n")
	}

	b.WriteString("\tquery += \" ORDER BY id\"\n\n")

	if hasFilters {
		b.WriteString("\trows, err := r.pool.Query(ctx, query, args...)\n")
	} else {
		b.WriteString("\trows, err := r.pool.Query(ctx, query)\n")
	}
	b.WriteString("\tif err != nil {\n")
	b.WriteString("\t\treturn nil, err\n")
	b.WriteString("\t}\n")
	b.WriteString("\tdefer rows.Close()\n\n")

	b.WriteString(fmt.Sprintf("\tvar results []%s.%s\n", modelsPkg, spec.Model))
	b.WriteString("\tfor rows.Next() {\n")
	b.WriteString(fmt.Sprintf("\t\tvar m %s.%s\n", modelsPkg, spec.Model))
	b.WriteString(fmt.Sprintf("\t\terr := rows.Scan(%s)\n", scanArgs(columns)))
	b.WriteString("\t\tif err != nil {\n")
	b.WriteString("\t\t\treturn nil, err\n")
	b.WriteString("\t\t}\n")
	b.WriteString("\t\tresults = append(results, m)\n")
	b.WriteString("\t}\n\n")

	b.WriteString("\treturn results, rows.Err()\n")
	b.WriteString("}\n\n")
}

// writeGetByIdentifiers generates GetByID and GetBy{Name} methods.
func writeGetByIdentifiers(b *strings.Builder, spec repo.RepoSpec, columns []columnInfo, table string, modelsPkg string) {
	idNames := sortedIdentifierNames(spec.Identifiers)
	for _, idName := range idNames {
		id := spec.Identifiers[idName]
		writeGetByIdentifier(b, spec, id, idName, columns, table, modelsPkg)
	}
}

func writeGetByIdentifier(b *strings.Builder, spec repo.RepoSpec, id repo.Identifier, idName string, columns []columnInfo, table string, modelsPkg string) {
	methodName := getByMethodName(idName)
	paramName := identifierParamName(idName, id)
	whereCol := identifierColumnName(id)

	b.WriteString(fmt.Sprintf("// %s retrieves a %s by %s.\n", methodName, spec.Model, idName))

	b.WriteString(fmt.Sprintf("func (r *%sRepository) %s(ctx context.Context, %s string) (*%s.%s, error) {\n",
		spec.Model, methodName, paramName, modelsPkg, spec.Model))

	cols := selectColumns(columns, "")
	b.WriteString(fmt.Sprintf("\tquery := `SELECT %s FROM %s WHERE %s = $1`\n\n",
		cols, table, whereCol))

	b.WriteString(fmt.Sprintf("\tvar m %s.%s\n", modelsPkg, spec.Model))
	b.WriteString(fmt.Sprintf("\terr := r.pool.QueryRow(ctx, query, %s).Scan(%s)\n", paramName, scanArgs(columns)))
	b.WriteString("\tif err != nil {\n")
	b.WriteString("\t\treturn nil, err\n")
	b.WriteString("\t}\n\n")

	b.WriteString("\treturn &m, nil\n")
	b.WriteString("}\n\n")
}

// writeCreate generates the Create method.
func writeCreate(b *strings.Builder, spec repo.RepoSpec, columns []columnInfo, table string, modelsPkg string) {
	b.WriteString(fmt.Sprintf("// Create inserts a new %s record.\n", spec.Model))
	b.WriteString(fmt.Sprintf("func (r *%sRepository) Create(ctx context.Context, input *%s.%s) (*%s.%s, error) {\n",
		spec.Model, modelsPkg, spec.Model, modelsPkg, spec.Model))

	// Build column list and parameter placeholders
	colNames := make([]string, len(columns))
	placeholders := make([]string, len(columns))
	inputArgs := make([]string, len(columns))
	for i, c := range columns {
		colNames[i] = c.columnName
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		inputArgs[i] = "input." + c.fieldName
	}

	colStr := strings.Join(colNames, ", ")
	phStr := strings.Join(placeholders, ", ")
	inputStr := strings.Join(inputArgs, ", ")
	retCols := selectColumns(columns, "")

	b.WriteString(fmt.Sprintf("\tquery := `INSERT INTO %s (%s) VALUES (%s) RETURNING %s`\n\n",
		table, colStr, phStr, retCols))

	b.WriteString(fmt.Sprintf("\tvar m %s.%s\n", modelsPkg, spec.Model))
	b.WriteString(fmt.Sprintf("\terr := r.pool.QueryRow(ctx, query, %s).Scan(%s)\n", inputStr, scanArgs(columns)))
	b.WriteString("\tif err != nil {\n")
	b.WriteString("\t\treturn nil, err\n")
	b.WriteString("\t}\n\n")

	b.WriteString("\treturn &m, nil\n")
	b.WriteString("}\n\n")
}

// writeUpdate generates the Update method.
func writeUpdate(b *strings.Builder, spec repo.RepoSpec, model yaml.Model, columns []columnInfo, table string, modelsPkg string) {
	b.WriteString(fmt.Sprintf("// Update modifies an existing %s record.\n", spec.Model))
	b.WriteString(fmt.Sprintf("func (r *%sRepository) Update(ctx context.Context, id string, input *%s.%s) (*%s.%s, error) {\n",
		spec.Model, modelsPkg, spec.Model, modelsPkg, spec.Model))

	// Find primary ID field name
	primaryField := "ID"
	if pid, ok := spec.Identifiers["primary"]; ok && len(pid.Fields) > 0 {
		primaryField = pid.Fields[0].Name
	}

	// Build SET clause: all columns except primary ID and CreatedAt
	var setCols []string
	var setArgs []string
	paramIdx := 1 // $1 is reserved for the WHERE id

	for _, c := range columns {
		if c.fieldName == primaryField || c.fieldName == "CreatedAt" {
			continue
		}
		paramIdx++
		setCols = append(setCols, fmt.Sprintf("%s = $%d", c.columnName, paramIdx))
		setArgs = append(setArgs, "input."+c.fieldName)
	}

	setStr := strings.Join(setCols, ", ")
	retCols := selectColumns(columns, "")
	whereCol := toSnakeCase(primaryField)

	// Build full args: id first, then set values
	allArgs := append([]string{"id"}, setArgs...)
	argStr := strings.Join(allArgs, ", ")

	b.WriteString(fmt.Sprintf("\tquery := `UPDATE %s SET %s WHERE %s = $1 RETURNING %s`\n\n",
		table, setStr, whereCol, retCols))

	b.WriteString(fmt.Sprintf("\tvar m %s.%s\n", modelsPkg, spec.Model))
	b.WriteString(fmt.Sprintf("\terr := r.pool.QueryRow(ctx, query, %s).Scan(%s)\n", argStr, scanArgs(columns)))
	b.WriteString("\tif err != nil {\n")
	b.WriteString("\t\treturn nil, err\n")
	b.WriteString("\t}\n\n")

	b.WriteString("\treturn &m, nil\n")
	b.WriteString("}\n\n")
}

// writeDelete generates the Delete method.
func writeDelete(b *strings.Builder, spec repo.RepoSpec, table string) {
	// Find primary ID column
	whereCol := "id"
	if pid, ok := spec.Identifiers["primary"]; ok && len(pid.Fields) > 0 {
		whereCol = toSnakeCase(pid.Fields[0].Name)
	}

	b.WriteString(fmt.Sprintf("// Delete removes a %s record by ID.\n", spec.Model))
	b.WriteString(fmt.Sprintf("func (r *%sRepository) Delete(ctx context.Context, id string) error {\n", spec.Model))
	b.WriteString(fmt.Sprintf("\tquery := `DELETE FROM %s WHERE %s = $1`\n\n", table, whereCol))
	b.WriteString("\t_, err := r.pool.Exec(ctx, query, id)\n")
	b.WriteString("\treturn err\n")
	b.WriteString("}\n")
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func getByMethodName(idName string) string {
	if idName == "primary" {
		return "GetByID"
	}
	return "GetBy" + upperFirst(idName)
}

func identifierParamName(idName string, id repo.Identifier) string {
	if idName == "primary" {
		return "id"
	}
	if len(id.Fields) > 0 {
		return lowerFirst(id.Fields[0].Name)
	}
	return idName
}

func identifierColumnName(id repo.Identifier) string {
	if len(id.Fields) > 0 {
		return toSnakeCase(id.Fields[0].Name)
	}
	return "id"
}

func sortedIdentifierNames(ids map[string]repo.Identifier) []string {
	names := make([]string, 0, len(ids))
	for name := range ids {
		names = append(names, name)
	}
	sort.SliceStable(names, func(i, j int) bool {
		if names[i] == "primary" {
			return true
		}
		if names[j] == "primary" {
			return false
		}
		return names[i] < names[j]
	})
	return names
}

func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	if strings.ToUpper(s) == s {
		return strings.ToLower(s)
	}
	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}
