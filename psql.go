// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// PostgreSQL-related
package main

import (
    "io"
    "fmt"
    "time"
    "strings"
    "strconv"
    "crypto/md5"
    "text/scanner"
    "database/sql"
    "encoding/json"
    _ "github.com/lib/pq"
)

// Database query
type DbQuery struct {
    Columns string              `json:"columns,omitempty"`
    Format string               `json:"format,omitempty"`
    Count bool                  `json:"count,omitempty"`
    Offset int                  `json:"offset,omitempty"`
    Limit int                   `json:"limit,omitempty"`
    NoHeader bool               `json:"noheader,omitempty"`
    Where string                `json:"where,omitempty"`
    Last string                 `json:"last,omitempty"`
    Order string                `json:"order,omitempty"`
    Descending bool             `json:"descending,omitempty"`
}

// Fields
const dbFieldSerial =       "serial"
const dbFieldModified =     "modified"
const dbFieldKey =          "key"
const dbFieldValue =        "value"
const dbColSeparator =      ";"

// Database globals
var sqlDB                   *sql.DB

// Open the database
func dbOpen() (err error) {

    if sqlDB != nil {
        return
    }

    sqlDB, err = sql.Open("postgres", ServiceConfig.SQLInfo)
    if err != nil {
        return
    }

    // Make sure the connection is alive
    sqlDB.Ping()
    if err != nil {
        return
    }

    return
}

// See if a table exists
func dbTableExists(tableName string) (exists bool, err error) {

    err = dbOpen()
    if err != nil {
        return
    }

    var row string
    query := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM pg_tables WHERE tablename = '%s')", tableName)
    err = sqlDB.QueryRow(query).Scan(&row)
    if err != nil {
        fmt.Printf("sql: error on table query: %s\n", err);
        return
    }
    if row != "true" && row != "t" {
        return
    }

    exists = true
    return

}

// Get the last serial sequence number of a table
func dbTableLatestSerial(tableName string) (serial int64, err error) {

    err = dbOpen()
    if err != nil {
        return
    }

    var row string
    query := fmt.Sprintf("SELECT max(%s) FROM %s", dbFieldSerial, tableName)
    err = sqlDB.QueryRow(query).Scan(&row)
    if err != nil {
        fmt.Printf("sql: error on table query: %s\n", err);
        return
    }
    serial, err = strconv.ParseInt(row, 10, 32)
    return

}

// Validate that the specified database table has been provisioned
func dbValidateTable(tableName string, provision bool) (err error) {

    // Make sure the DB is open
    err = dbOpen()
    if err != nil {
        return
    }

    // Make sure the table exists
    exists, err := dbTableExists(tableName)
    if exists {
        return
    }

    // If we don't want to provision it, bail
    if !provision {
        return fmt.Errorf("table not found: %s", tableName);
    }

    // Create the table
    query := fmt.Sprintf("CREATE TABLE \"%s\" ( \n", tableName)
    query += fmt.Sprintf("%s BIGSERIAL NOT NULL UNIQUE, \n", dbFieldSerial)
    query += fmt.Sprintf("%s TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT (current_timestamp AT TIME ZONE 'UTC'), \n", dbFieldModified)
    query += fmt.Sprintf("%s TEXT PRIMARY KEY, \n", dbFieldKey)
    query += fmt.Sprintf("%s JSONB \n", dbFieldValue)
    query += "); \n"
    rows, err := sqlDB.Query(query)
    if err != nil {
        err = fmt.Errorf("table creation error: %s", err)
        return
    }
    defer rows.Close()

    // Done
    fmt.Printf("sql: table %s created\n", tableName)
    return

}

// Perform an arbitrary query on the table, and display the results
func dbQuery(query string) (err error) {

    err = dbOpen()
    if err != nil {
        return
    }

    rows, err := sqlDB.Query(query)
    if err != nil {
        return
    }
    defer rows.Close()

    return
}

// Drop the table
func dbDrop(tableName string) (err error) {

    err = dbValidateTable(tableName, false)
    if err != nil {
        return
    }

    rows, err := sqlDB.Query(fmt.Sprintf("drop table \"%s\"", tableName))
    if err != nil {
        return
    }
    defer rows.Close()

    return

}

// Add an object to the database
func dbAddObject(tableName string, key string, object interface{}) (err error) {

    // Create a unique ID for the record if not specified
    if key == "" {
        randstr := fmt.Sprintf("%d", time.Now().UnixNano())
        randstr += fmt.Sprintf(".%d", Random(0, 1000000000))
        randstr += fmt.Sprintf(".%d", Random(0, 1000000000))
        randstr += fmt.Sprintf(".%d", Random(0, 1000000000))
        key = dbHashKey(randstr)
    }

    objJSON, err := json.Marshal(object)
    if err != nil {
        return
    }

    // Make sure that the table is valid
    err = dbValidateTable(tableName, false)
    if err != nil {
        return
    }

    // Quote the single-quotes in the string because of SQL restrictions
    value := string(objJSON)
    jsonString := strings.Replace(value, "'", "''", -1)

    // Add it to the database
    query := fmt.Sprintf("INSERT INTO \"%s\" (%s,%s) VALUES ('%s','%s')", tableName, dbFieldKey, dbFieldValue, key, jsonString)
    rows, err := sqlDB.Query(query)
    if err != nil {
        return
    }
    defer rows.Close()

    // Done
    return

}

// Fetch an object from the database
func dbGetObject(tableName string, key string, object interface{}) (exists bool, err error) {

    var valueStr string
    query := fmt.Sprintf("SELECT %s FROM \"%s\" WHERE (%s = '%s') LIMIT 1", dbFieldValue, tableName, dbFieldKey, key)
    err = sqlDB.QueryRow(query).Scan(&valueStr)
    if err != nil {
        err = nil
        return
    }

    exists = true

    err = json.Unmarshal([]byte(valueStr), object)
    if err != nil {
        return
    }

    return

}

// Update an object in the database
func dbUpdateObject(tableName string, key string, object interface{}) (err error) {

    objJSON, err := json.Marshal(object)
    if err != nil {
        return
    }

    // Quote the single-quotes in the string because of SQL restrictions
    jsonString := strings.Replace(string(objJSON), "'", "''", -1)

    // Do the update
    query := fmt.Sprintf("UPDATE \"%s\" SET %s = '%s', %s = clock_timestamp() WHERE %s = '%s'", tableName, dbFieldValue, jsonString, dbFieldModified, dbFieldKey, key)
    rows, err := sqlDB.Query(query)
    if err != nil {
        return
    }
    defer rows.Close()

    // Done
    return

}

// Delete a record in the database, if this is a later sequence number
func dbDelete(tableName string, key string) (err error) {

    // Make sure that DB is open
    err = dbValidateTable(tableName, false)
    if err != nil {
        return
    }

    // Perform the query
    query := fmt.Sprintf("DELETE FROM \"%s\" WHERE %s = '%s'", tableName, dbFieldKey, key)
    rows, err := sqlDB.Query(query)
    if err != nil {
        return
    }
    defer rows.Close()

    // Done
    return

}

// Compute a hashed key for the specified variable-length string
func dbHashKey(stringToHash string) (key string) {
    h := md5.New()
    io.WriteString(h, stringToHash)
    hexHash := fmt.Sprintf("%x", h.Sum(nil))
    return hexHash
}

// Perform a query and output to an Writer response writer
func dbQueryToWriter(writer io.Writer, query string, serialCol bool, q *DbQuery) (serial int64, response string, err error) {

    // Special handling to display all tables
    var rows *sql.Rows
    fmt.Printf("ozzie raw sql in:\n'%s'\n", query)
    rows, err = sqlDB.Query(query)
    fmt.Printf("ozzie raw sql bck:\n'%s'\n", query)
    if err != nil {
        return
    }
    defer rows.Close()

    // Do special handling of "count"
    if q.Count {

        // Skip the first row containing the column name "count"
        if !rows.Next() {
            err = fmt.Errorf("query count error: missing column header")
            return
        }

        // Interpret the next row as a number
        err = rows.Scan(&response)
        if err != nil {
            err = fmt.Errorf("query count error: %s", err)
            return
        }

        // Done
        return

    }

    // Dispatch based on format
    if q.Format == "" || strings.ToLower(q.Format) == "json" {
        serial, err = dbQueryWriterInJSON(writer, rows, serialCol, q)
        return
    } else if strings.ToLower(q.Format) == "csv" {
        serial, err = dbQueryWriterInCSV(writer, rows, serialCol, q)
        return
    }

    err = fmt.Errorf("unrecognized format: %s", q.Format)
    return

}

// Do query to CSV
func dbQueryWriterInCSV(writer io.Writer, rows *sql.Rows, serialCol bool, q *DbQuery) (serial int64, err error) {

    // Suppress the header if it's a count value
    if q.Count {
        q.NoHeader = true
    }

    // Output the column names
    cols, err := rows.Columns()
    if err != nil {
        return
    }
    keyColumn := 99999
    _ = keyColumn
    valueColumn := 99999
    _ = valueColumn
    columnsOutput := 0
    for i, colName := range(cols) {
        if colName == dbFieldKey {
            keyColumn = i
            continue
        }
        if colName == dbFieldValue {
            valueColumn = i
        }
        if !q.NoHeader {
            if columnsOutput != 0 {
                io.WriteString(writer, ",")
            }
            io.WriteString(writer, colName)
            columnsOutput++
        }
    }
    if !q.NoHeader {
        io.WriteString(writer, "\n")
    }

    // Create an array to contain the columns
    rawColArray := make([][]byte, len(cols))
    dest := make([]interface{}, len(cols))
    for i := range rawColArray {
        dest[i] = &rawColArray[i]
    }

    // Iterate, outputing them
    for rows.Next() {
        columnsOutput = 0
        err = rows.Scan(dest...)
        if err != nil {
            break
        }
        for i, raw := range rawColArray {
            // Do special processing for serial, in which we
            // optimistically decode the first column to see if it
            // is a number.  If so, the caller is interested in
            // the very last value.
            if serialCol && i == 0 {
                i64, err := strconv.ParseInt(string(raw), 10, 32)
                if err == nil {
                    serial = i64
                }
                continue
            }
            if columnsOutput != 0 {
                io.WriteString(writer, ",")
            }
            if raw != nil {
                io.WriteString(writer, string(raw))
            }
            columnsOutput++
        }
        io.WriteString(writer, "\n")
    }

    return
}

// Do query to JSON
func dbQueryWriterInJSON(writer io.Writer, rows *sql.Rows, serialCol bool, q *DbQuery) (serial int64, err error) {

    // Output the column names
    cols, err := rows.Columns()
    if err != nil {
        return
    }
    keyColumn := 99999
    _ = keyColumn
    valueColumn := 99999
    _ = valueColumn
    columnsOutput := 0
    for i, colName := range(cols) {
        if colName == dbFieldKey {
            keyColumn = i
            continue
        }
        if colName == dbFieldValue {
            valueColumn = i
        }
    }
    isJustValue := (len(cols) == 1 && valueColumn == 0)

    io.WriteString(writer, "[\n")

    // Create an array to contain the columns
    rawColArray := make([][]byte, len(cols))
    dest := make([]interface{}, len(cols))
    for i := range rawColArray {
        dest[i] = &rawColArray[i]
    }

    // Iterate, outputing them
    rowsOutput := 0
    for rows.Next() {
        columnsOutput = 0
        err = rows.Scan(dest...)
        if err != nil {
            break
        }
        if rowsOutput != 0 {
            io.WriteString(writer, ",\n")
        }
        if (!isJustValue) {
            io.WriteString(writer, "{")
        }
        for i, raw := range rawColArray {
            // Do special processing for serial, in which we
            // optimistically decode the first column to see if it
            // is a number.  If so, the caller is interested in
            // the very last value.
            if serialCol && i == 0 {
                i64, err := strconv.ParseInt(string(raw), 10, 32)
                if err == nil {
                    serial = i64
                }
                continue
            }
            if raw != nil {
                if columnsOutput != 0 {
                    io.WriteString(writer, ",")
                }
                if (isJustValue) {
                    io.WriteString(writer, fmt.Sprintf("%s", string(raw)))
                } else {
                    io.WriteString(writer, fmt.Sprintf("\"%s\":%s", cols[i], string(raw)))
                }
                columnsOutput++
            }
        }
        if (!isJustValue) {
            io.WriteString(writer, "}")
        }
        rowsOutput++
    }

    if rowsOutput != 0 {
        io.WriteString(writer, "\n")
    }
    io.WriteString(writer, "]\n")

    return
}

// Build a SQL query from arguments
func dbBuildQuery(tableName string, q *DbQuery) (query string, err error) {

    // Break down the columns into []strings
    colField := []string{}
    colDisplay := []string{}
    numCols := 0
    if q.Columns != "" {
        for colNo, col := range strings.Split(q.Columns, dbColSeparator) {

            // Break it into display name and expression
            components := strings.SplitN(col, ":", 2)
            display := ""
            expression := components[0]
            if len(components) == 2 {
                display = components[0]
                expression = components[1]
            }

            // Map the column expression to a field value
            field, label, err2 := filterExpression(expression)
            if err2 != nil {
                err = err2
                return
            }

            // Set the label if not already set
            if display == "" {
                display = label
            }
            if display == "" {
                display = fmt.Sprintf("col%d", colNo+1)
            }

            // Append to results
            colField = append(colField, field)
            colDisplay = append(colDisplay, display)
            numCols++

        }
    }

    // Exit if count combined with field names
    if q.Count {
        if numCols != 0 || q.Limit != 0 || q.Offset != 0 {
            err = fmt.Errorf("'count' may not be combined with 'columns' or 'offset' or 'limit'")
            return
        }
    }

    // Exit if no columns specified
    if !q.Count && numCols == 0 {
        err = fmt.Errorf("at least one 'column' must be requested within the query")
        return
    }

    // Build the query
    query += "SELECT "
    if q.Count {
        query += "COUNT(*)"
    } else {
        for i:=0; i<numCols; i++ {
            if i != 0 {
                query += ", "
            }
            query += colField[i] + " AS \"" + colDisplay[i] + "\""
        }
    }
    query += " FROM " + tableName
    if q.Last != "" {
        clean := ""
        clean, err = filterLast(q.Last)
        if err != nil {
            return
        }
        query += fmt.Sprintf(" WHERE (%s)", clean)
    }
    if q.Order != "" {
        clean, _, err2 := filterExpression(q.Order)
        if err2 != nil {
            err = err2
            return
        }
        query += fmt.Sprintf(" ORDER BY (%s)", clean)
        if q.Descending {
            query += " DESC"
        } else {
            query += " ASC"
        }
    }
    if q.Limit != 0 {
        query += fmt.Sprintf(" LIMIT %d", q.Limit)
    }
    if q.Offset != 0 {
        query += fmt.Sprintf(" OFFSET %d", q.Offset)
    }

    // Debug to watch the queries go by
    if (false) {
        fmt.Printf("QUERY: %s\n", query)
    }

    return

}

// Filter the 'last' specifier
func filterLast(lastIn string) (whereOut string, err error) {

    word := ""
    numstr := ""
    if strings.HasSuffix(lastIn, "y") {
        word = "year"
        numstr = strings.TrimSuffix(lastIn, "y")
    } else if strings.HasSuffix(lastIn, "m") {
        word = "month"
        numstr = strings.TrimSuffix(lastIn, "m")
    } else if strings.HasSuffix(lastIn, "w") {
        word = "week"
        numstr = strings.TrimSuffix(lastIn, "w")
    } else if strings.HasSuffix(lastIn, "d") {
        word = "day"
        numstr = strings.TrimSuffix(lastIn, "d")
    } else if strings.HasSuffix(lastIn, "h") {
        word = "hour"
        numstr = strings.TrimSuffix(lastIn, "h")
    } else if strings.HasSuffix(lastIn, "m") {
        word = "minute"
        numstr = strings.TrimSuffix(lastIn, "m")
    } else if strings.HasSuffix(lastIn, "s") {
        word = "second"
        numstr = strings.TrimSuffix(lastIn, "s")
    } else {
        err = fmt.Errorf("cannot interpret time unit suffix: %s", lastIn)
        return
    }
    i64, err := strconv.ParseInt(numstr, 10, 32)
    if err != nil {
        return
    }
    if i64 > 1 {
        word += "s"
    }

    whereOut = fmt.Sprintf("modified >= now() - interval '%s %s'", numstr, word)
    return

}

// Filter an expression specifier both so that it's safe and to
// map field names and SQL formats properly.  This is used both
// in "where" and also in column expressions
func filterExpression(whereIn string) (whereOut string, label string, err error) {
    var s scanner.Scanner

    s.Init(strings.NewReader(whereIn))
    s.Error = filterErrorHandler
    s.IsIdentRune = filterIsIdent

    for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {

        if false {
            fmt.Printf("token(%s):'%s' next:'%c'\n", scanner.TokenString(tok), s.TokenText(), s.Peek())
        }

        switch scanner.TokenString(tok) {

        case "Ident":
            var str string
            // If the very next thing is a type cast, textify the jsonb so that we don't get a
            // query error because it doesn't allow coercion of a jsonb to a type other than text.
            var lbl string
            textify := s.Peek() == ':'
            str, lbl, err = filterMapIdent(s.TokenText(), textify)
            if err != nil {
                return
            }
            if lbl != "" && label == "" {
                label = lbl
            }
            whereOut += str

        case "Float":
            fallthrough
        case "Int":
            fallthrough
        case "String":
            whereOut += s.TokenText()

        case "char":
            whereOut += s.TokenText()

        default:
            whereOut += s.TokenText()

        }

        if fmt.Sprintf("%c", s.Peek()) == " " {
            whereOut += " "
        }

    }

    return

}

// Error handler, necessary to suppress errors during scan
func filterErrorHandler(s *scanner.Scanner, msg string) {
}

// Determine which characters are valid in an identifier
func filterIsIdent(ch rune, i int) bool {
    valid := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_."
    if i == 0 {
        valid = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_."
    }
    return strings.Contains(valid, fmt.Sprintf("%c", ch))
}

// Map an identifier to safe 'where' syntax
func filterMapIdent(ident string, textify bool) (field string, label string, err error) {

    sep := "->"
    if textify {
        sep = "->>"
    }

    switch ident {

        // This is because this is so very common
    case "q":
        fallthrough
    case "quote":
        field = "quote_ident"

    case ".serial":
        label = "serial"
        field = dbFieldSerial

    case ".modified":
        label = "modified"
        field = dbFieldModified

    case ".value":
        label = "value"
        field = dbFieldValue

    default:
        if strings.HasPrefix(ident, ".value.") {
            s0 := strings.TrimPrefix(ident, ".value.")
            s1 := strings.Split(s0, ".")
            field = "(" + dbFieldValue
            if len(s1) == 1 {
                label = s1[0]
                field += sep + "'"
                field += s1[0]
                field += "'"
            } else if len(s1) == 2 {
                label = s1[1]
                field += "->'"
                field += s1[0]
                field += "'->>'"
                field += s1[1]
                field += "'"
            } else {
                label = s1[len(s1)-1]
                s2 := s1[:len(s1)-1]
                field += "->'"
                field += strings.Join(s2, "'->'")
                field += "'" + sep + "'"
                field += s1[len(s1)-1]
                field += "'"
            }
            field += ")"
        } else if strings.HasPrefix(ident, ".") {
            err = fmt.Errorf("unrecognized keyword: %s", ident)
            return
        } else {
            field = ident
        }

    }

    return

}
