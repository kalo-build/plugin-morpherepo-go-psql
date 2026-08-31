package compile

import (
	"fmt"
	"strings"

	"github.com/kalo-build/morphe-go/pkg/yaml"
	"github.com/kalo-build/plugin-morpherepo-go/pkg/repo"
)

// relationInfo describes a ForOne relation for Query generation.
type relationInfo struct {
	fieldName     string       // Go field name on parent struct (e.g., "Organization")
	fkColumn      string       // FK column on root table (e.g., "organization_id")
	fkFieldName   string       // Go FK field name (e.g., "OrganizationID")
	targetModel   string       // Target model name (e.g., "Organization")
	targetTable   string       // Target table name (e.g., "organizations")
	targetAlias   string       // SQL alias (e.g., "o")
	targetColumns []columnInfo // Target model's direct columns
}

// extractForOneRelations extracts ForOne relation metadata from a model,
// looking up target model columns from the registry.
func extractForOneRelations(model yaml.Model, allModels map[string]yaml.Model, allEnums map[string]yaml.Enum, usedAliases map[string]bool) []relationInfo {
	var relations []relationInfo

	relNames := sortedMapKeys(model.Related)
	for _, relName := range relNames {
		rel := model.Related[relName]
		if rel.Type != "ForOne" {
			continue
		}

		targetModelName := relName

		targetModel, ok := allModels[targetModelName]
		if !ok {
			continue
		}

		var targetCols []columnInfo
		fieldNames := sortedMapKeys(targetModel.Fields)
		for _, fname := range fieldNames {
			field := targetModel.Fields[fname]
			isOptional := false
			for _, attr := range field.Attributes {
				if attr == "optional" {
					isOptional = true
					break
				}
			}
			col := columnInfo{
				columnName: toSnakeCase(fname),
				fieldName:  fname,
				isPointer:  isOptional,
				morpheType: string(field.Type),
			}
			if allEnums != nil && !yaml.IsModelFieldTypePrimitive(field.Type) {
				if enum, ok := allEnums[string(field.Type)]; ok {
					col.columnName = toSnakeCase(fname) + "_id"
					col.isEnum = true
					col.enumTableName = pluralize(toSnakeCase(enum.Name))
				}
			}
			targetCols = append(targetCols, col)
		}

		targetTable := tableName(targetModelName)
		alias := assignTableAlias(targetTable, usedAliases)

		relations = append(relations, relationInfo{
			fieldName:     relName,
			fkColumn:      toSnakeCase(relName) + "_id",
			fkFieldName:   relName + "ID",
			targetModel:   targetModelName,
			targetTable:   targetTable,
			targetAlias:   alias,
			targetColumns: targetCols,
		})
	}

	return relations
}

// assignTableAlias picks a short SQL alias for a table, avoiding conflicts.
func assignTableAlias(tbl string, used map[string]bool) string {
	// Try first letter
	alias := string(tbl[0])
	if !used[alias] {
		used[alias] = true
		return alias
	}
	// Try first two letters
	if len(tbl) >= 2 {
		alias = tbl[:2]
		if !used[alias] {
			used[alias] = true
			return alias
		}
	}
	// Fallback: first letter + number
	for i := 2; ; i++ {
		alias = fmt.Sprintf("%c%d", tbl[0], i)
		if !used[alias] {
			used[alias] = true
			return alias
		}
	}
}

// writeQuery generates the Query method for query-by-example.
func writeQuery(b *strings.Builder, spec repo.RepoSpec, model yaml.Model, columns []columnInfo, table string, modelsPkg string, allModels map[string]yaml.Model, allEnums map[string]yaml.Enum) {
	structName := spec.Model + "Repository"

	usedAliases := map[string]bool{}
	rootAlias := assignTableAlias(table, usedAliases)
	relations := extractForOneRelations(model, allModels, allEnums, usedAliases)

	b.WriteString(fmt.Sprintf("// Query finds %s records matching the non-zero fields of the example struct.\n", spec.Model))
	b.WriteString("// Non-nil relation pointers trigger JOINs; populated fields on relations add WHERE conditions.\n")
	b.WriteString(fmt.Sprintf("func (r *%s) Query(ctx context.Context, example *%s.%s) ([]%s.%s, error) {\n",
		structName, modelsPkg, spec.Model, modelsPkg, spec.Model))

	b.WriteString("\tselectCols := []string{")
	for i, c := range columns {
		if i > 0 {
			b.WriteString(", ")
		}
		if c.isEnum {
			b.WriteString(fmt.Sprintf("%q", fmt.Sprintf("(SELECT value FROM %s WHERE id = %s.%s)", c.enumTableName, rootAlias, c.columnName)))
		} else {
			b.WriteString(fmt.Sprintf("%q", rootAlias+"."+c.columnName))
		}
	}
	b.WriteString("}\n")

	if len(relations) > 0 {
		b.WriteString("\tjoins := []string{}\n")
	}
	b.WriteString("\tconditions := []string{}\n")
	b.WriteString("\targs := []interface{}{}\n")
	b.WriteString("\tparamIdx := 1\n\n")

	for _, c := range columns {
		check := zeroValueCheck("example", c.fieldName, c.isPointer, c.morpheType)
		if check == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("\tif %s {\n", check))
		if c.isEnum {
			b.WriteString(fmt.Sprintf("\t\tconditions = append(conditions, fmt.Sprintf(\"%s.%s = (SELECT id FROM %s WHERE value = $%%d)\", paramIdx))\n", rootAlias, c.columnName, c.enumTableName))
		} else {
			b.WriteString(fmt.Sprintf("\t\tconditions = append(conditions, fmt.Sprintf(\"%s.%s = $%%d\", paramIdx))\n", rootAlias, c.columnName))
		}
		b.WriteString(fmt.Sprintf("\t\targs = append(args, %s)\n", argValue("example", c.fieldName, c.isPointer)))
		b.WriteString("\t\tparamIdx++\n")
		b.WriteString("\t}\n")
	}

	// Relation checks
	for _, rel := range relations {
		b.WriteString(fmt.Sprintf("\n\tscan%s := false\n", rel.fieldName))
		b.WriteString(fmt.Sprintf("\tif example.%s != nil {\n", rel.fieldName))
		b.WriteString(fmt.Sprintf("\t\tscan%s = true\n", rel.fieldName))

		// Find the primary ID column of the target model for the JOIN ON clause
		targetPKColumn := "id"
		if tm, ok := allModels[rel.targetModel]; ok {
			if pid, hasPrimary := tm.Identifiers["primary"]; hasPrimary && len(pid.Fields) > 0 {
				targetPKColumn = toSnakeCase(pid.Fields[0])
			}
		}

		b.WriteString(fmt.Sprintf("\t\tjoins = append(joins, \"JOIN %s %s ON %s.%s = %s.%s\")\n",
			rel.targetTable, rel.targetAlias,
			rootAlias, rel.fkColumn,
			rel.targetAlias, targetPKColumn))

		b.WriteString("\t\tselectCols = append(selectCols, ")
		for i, tc := range rel.targetColumns {
			if i > 0 {
				b.WriteString(", ")
			}
			if tc.isEnum {
				b.WriteString(fmt.Sprintf("%q", fmt.Sprintf("(SELECT value FROM %s WHERE id = %s.%s)", tc.enumTableName, rel.targetAlias, tc.columnName)))
			} else {
				b.WriteString(fmt.Sprintf("%q", rel.targetAlias+"."+tc.columnName))
			}
		}
		b.WriteString(")\n")

		for _, tc := range rel.targetColumns {
			check := zeroValueCheck(fmt.Sprintf("example.%s", rel.fieldName), tc.fieldName, tc.isPointer, tc.morpheType)
			if check == "" {
				continue
			}
			b.WriteString(fmt.Sprintf("\t\tif %s {\n", check))
			if tc.isEnum {
				b.WriteString(fmt.Sprintf("\t\t\tconditions = append(conditions, fmt.Sprintf(\"%s.%s = (SELECT id FROM %s WHERE value = $%%d)\", paramIdx))\n", rel.targetAlias, tc.columnName, tc.enumTableName))
			} else {
				b.WriteString(fmt.Sprintf("\t\t\tconditions = append(conditions, fmt.Sprintf(\"%s.%s = $%%d\", paramIdx))\n", rel.targetAlias, tc.columnName))
			}
			b.WriteString(fmt.Sprintf("\t\t\targs = append(args, %s)\n", argValue(fmt.Sprintf("example.%s", rel.fieldName), tc.fieldName, tc.isPointer)))
			b.WriteString("\t\t\tparamIdx++\n")
			b.WriteString("\t\t}\n")
		}

		b.WriteString("\t}\n")
	}

	// Build query string
	b.WriteString(fmt.Sprintf("\n\tquery := \"SELECT \" + strings.Join(selectCols, \", \") + \" FROM %s %s\"\n", table, rootAlias))
	if len(relations) > 0 {
		b.WriteString("\tfor _, j := range joins {\n")
		b.WriteString("\t\tquery += \" \" + j\n")
		b.WriteString("\t}\n")
	}
	b.WriteString("\tif len(conditions) > 0 {\n")
	b.WriteString("\t\tquery += \" WHERE \" + strings.Join(conditions, \" AND \")\n")
	b.WriteString("\t}\n")
	b.WriteString(fmt.Sprintf("\tquery += \" ORDER BY %s.id\"\n\n", rootAlias))

	// Execute query
	b.WriteString("\trows, err := r.pool.Query(ctx, query, args...)\n")
	b.WriteString("\tif err != nil {\n")
	b.WriteString("\t\treturn nil, err\n")
	b.WriteString("\t}\n")
	b.WriteString("\tdefer rows.Close()\n\n")

	// Scan results
	b.WriteString(fmt.Sprintf("\tvar results []%s.%s\n", modelsPkg, spec.Model))
	b.WriteString("\tfor rows.Next() {\n")
	b.WriteString(fmt.Sprintf("\t\tvar m %s.%s\n", modelsPkg, spec.Model))

	// Build scan destination
	b.WriteString("\t\tscanDest := []interface{}{")
	for i, c := range columns {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("&m." + c.fieldName)
	}
	b.WriteString("}\n")

	// Conditional relation scan destinations
	for _, rel := range relations {
		varName := "rel" + rel.fieldName
		b.WriteString(fmt.Sprintf("\n\t\tvar %s %s.%s\n", varName, modelsPkg, rel.targetModel))
		b.WriteString(fmt.Sprintf("\t\tif scan%s {\n", rel.fieldName))
		b.WriteString("\t\t\tscanDest = append(scanDest, ")
		for i, tc := range rel.targetColumns {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("&%s.%s", varName, tc.fieldName))
		}
		b.WriteString(")\n")
		b.WriteString("\t\t}\n")
	}

	// Scan
	b.WriteString("\n\t\tif err := rows.Scan(scanDest...); err != nil {\n")
	b.WriteString("\t\t\treturn nil, err\n")
	b.WriteString("\t\t}\n")

	// Populate relation pointers
	for _, rel := range relations {
		varName := "rel" + rel.fieldName
		b.WriteString(fmt.Sprintf("\n\t\tif scan%s {\n", rel.fieldName))
		b.WriteString(fmt.Sprintf("\t\t\tm.%s = &%s\n", rel.fieldName, varName))
		b.WriteString("\t\t}\n")
	}

	b.WriteString("\n\t\tresults = append(results, m)\n")
	b.WriteString("\t}\n\n")

	b.WriteString("\treturn results, rows.Err()\n")
	b.WriteString("}\n\n")
}

// writeQueryOne generates the QueryOne method.
func writeQueryOne(b *strings.Builder, spec repo.RepoSpec, modelsPkg string) {
	structName := spec.Model + "Repository"

	b.WriteString(fmt.Sprintf("// QueryOne finds exactly one %s record matching the example.\n", spec.Model))
	b.WriteString("// Returns pgx.ErrNoRows if no match, error if more than one match.\n")
	b.WriteString(fmt.Sprintf("func (r *%s) QueryOne(ctx context.Context, example *%s.%s) (*%s.%s, error) {\n",
		structName, modelsPkg, spec.Model, modelsPkg, spec.Model))
	b.WriteString("\tresults, err := r.Query(ctx, example)\n")
	b.WriteString("\tif err != nil {\n")
	b.WriteString("\t\treturn nil, err\n")
	b.WriteString("\t}\n")
	b.WriteString("\tif len(results) == 0 {\n")
	b.WriteString("\t\treturn nil, pgx.ErrNoRows\n")
	b.WriteString("\t}\n")
	b.WriteString("\tif len(results) > 1 {\n")
	b.WriteString(fmt.Sprintf("\t\treturn nil, fmt.Errorf(\"QueryOne %s: expected 1 result, got %%d\", len(results))\n", spec.Model))
	b.WriteString("\t}\n")
	b.WriteString("\treturn &results[0], nil\n")
	b.WriteString("}\n")
}

// zeroValueCheck generates a Go expression that checks if a field is non-zero.
func zeroValueCheck(receiver string, fieldName string, isPointer bool, morpheType string) string {
	accessor := receiver + "." + fieldName
	if isPointer {
		return fmt.Sprintf("%s != nil", accessor)
	}
	switch morpheType {
	case "UUID", "String", "":
		return fmt.Sprintf("%s != \"\"", accessor)
	case "Integer", "AutoIncrement":
		return fmt.Sprintf("%s != 0", accessor)
	case "Float":
		return fmt.Sprintf("%s != 0", accessor)
	case "Timestamp", "Time", "Date":
		return fmt.Sprintf("!%s.IsZero()", accessor)
	case "Boolean":
		return "" // skip — ambiguous zero value
	default:
		return fmt.Sprintf("%s != \"\"", accessor)
	}
}

// argValue generates the Go expression for the argument value.
func argValue(receiver string, fieldName string, isPointer bool) string {
	if isPointer {
		return "*" + receiver + "." + fieldName
	}
	return receiver + "." + fieldName
}
