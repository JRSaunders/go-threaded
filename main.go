package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	"net"
	"sync"
)

type NodeQueries struct {
	NodeQueries []*NodeQuery `json:"node_queries"`
}
type Dsn struct {
	Driver   string `json:"driver"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Dbname   string `json:"dbname"`
	User     string `json:"user"`
	Password string `json:"password"`
	CharSet  string `json:"charset"`
}

type Node struct {
	Name string `json:"name"`
	Dsn  Dsn    `json:"dsn"`
	Geo  string `json:"geo"`
}

type NodeQuery struct {
	Node            Node         `json:"node"`
	Sql             string       `json:"sql"`
	Binds           []BindString `json:"binds"`
	JsonReturnBytes []interface{}
}
type BindString struct {
	Value string `json:"value"`
	Key   string `json:"key"`
}
type ReturnDataNodes struct {
	Nodes []ReturnData `json:"nodes"`
}

type ReturnData struct {
	NodeName string        `json:"node_name""`
	Data     []interface{} `json:"data"`
}

func main() {
	ln, err := net.Listen("tcp", "localhost:1534")
	if err != nil {
		fmt.Print(err)
	}
	fmt.Println(ln.Addr().String() + `: Ready to receive connections`)
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Print(err)
		}
		fmt.Println(conn.RemoteAddr().String() + `: Connected`)
		go handleConnection(conn)
	}
}

func runQuery(wg *sync.WaitGroup, nodeQuery *NodeQuery) {

	finalRows := []interface{}{}
	nodeDsn := nodeQuery.Node.Dsn
	dsn := nodeDsn.User + ":" + nodeDsn.Password +
		"@(" + nodeDsn.Host + ":" + nodeDsn.Port +
		")/" + nodeDsn.Dbname
	db, err := sql.Open(nodeDsn.Driver, dsn)

	if err != nil {
		panic(err)
	}
	defer db.Close()

	bindCount := len(nodeQuery.Binds)
	params := make([]interface{}, bindCount)
	for i, v := range nodeQuery.Binds {
		params[i] = string(v.Value)
		fmt.Println("DID SHIT" + v.Value)
	}
	//m := []string{"2020-04-23 11:58:10", "2020-04-21 11:58:10"}
	fmt.Printf("%v", params)

	params2 := make([]interface{}, 2)
	params2[0] = "2020-04-23 11:58:10"
	params2[1] = "2020-04-23 11:58:10"

	rows, err := db.Query(nodeQuery.Sql, pq.Array(params2))
	if err != nil {
		fmt.Println(err.Error())
		wg.Done()
		return
	}
	columnTypes, err := rows.ColumnTypes()

	if err != nil {
		fmt.Println(err.Error())
		wg.Done()
		return
	}

	count := len(columnTypes)

	for rows.Next() {

		scanArgs := make([]interface{}, count)

		for i, v := range columnTypes {

			switch v.DatabaseTypeName() {
			case "VARCHAR", "TEXT", "UUID", "TIMESTAMP", "DATETIME", "DATE":
				scanArgs[i] = new(sql.NullString)
				break
			case "BOOL":
				scanArgs[i] = new(sql.NullBool)
				break
			case "INT4":
				scanArgs[i] = new(sql.NullInt64)
				break
			default:
				scanArgs[i] = new(sql.NullString)
			}
		}

		err := rows.Scan(scanArgs...)

		if err != nil {
			fmt.Println(err.Error())
			wg.Done()
			return
		}

		masterData := map[string]interface{}{}

		for i, v := range columnTypes {

			if z, ok := (scanArgs[i]).(*sql.NullBool); ok {
				masterData[v.Name()] = z.Bool
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullString); ok {
				masterData[v.Name()] = z.String
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullInt64); ok {
				masterData[v.Name()] = z.Int64
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullFloat64); ok {
				masterData[v.Name()] = z.Float64
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullInt32); ok {
				masterData[v.Name()] = z.Int32
				continue
			}

			masterData[v.Name()] = scanArgs[i]
		}

		finalRows = append(finalRows, masterData)
	}
	db.Close()

	nodeQuery.JsonReturnBytes = finalRows

	wg.Done()
}

func handleConnection(conn net.Conn) {

	var response [2048]byte
	n, _ := conn.Read(response[0:])
	s := string(response[0:n])

	var nq NodeQueries

	json.Unmarshal([]byte(s), &nq)
	var wg sync.WaitGroup
	for _, nodeQuery := range nq.NodeQueries {
		wg.Add(1)
		go runQuery(&wg, nodeQuery)

	}
	wg.Wait()
	fmt.Println("Done")
	data := ReturnDataNodes{}

	for _, nodeQuery := range nq.NodeQueries {
		data.Nodes = append(data.Nodes, ReturnData{
			NodeName: nodeQuery.Node.Name,
			Data:     nodeQuery.JsonReturnBytes,
		})
	}
	y, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err.Error())
	}
	conn.Write(y)
	conn.Close()
}
