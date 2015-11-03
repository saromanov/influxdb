package main

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"
	"time"
)

////////////////////////////////////

//type Tag struct {
//	Key   []byte
//	Value []byte
//}
//
//type Field struct {
//	Key   []byte
//	Value []byte
//}
//type Point struct {
//	Measurement []byte
//	Tags        []Tag
//	Fields      []Field
//	Timestamp   []byte
//}

// Tag is a struct for a tag in influxdb
type Tag struct {
	Key   string
	Value string
}

// Field is a struct for a field in influxdb
type Field struct {
	Key   string
	Value string
}

// Tags is an slice of all the tags for a point
type Tags []Tag

// Fields is an slice of all the fields for a point
type Fields []Field

// Point represents a point in InfluxDB
type Point struct {
	Measurement string
	Tags        Tags
	Fields      Fields
	Timestamp   int64
}

// tagset returns a byte array for a points tagset
func (t Tags) tagset() []byte {
	var buf bytes.Buffer
	for _, tag := range t {
		buf.Write([]byte(fmt.Sprintf("%v=%v,", tag.Key, tag.Value)))
	}

	b := buf.Bytes()
	b = b[0 : len(b)-1]

	return b
}

// fieldset returns a byte array for a points fieldset
func (f Fields) fieldset() []byte {
	var buf bytes.Buffer
	for _, field := range f {
		buf.Write([]byte(fmt.Sprintf("%v=%v,", field.Key, field.Value)))
	}

	b := buf.Bytes()
	b = b[0 : len(b)-1]

	return b
}

// Line returns a byte array for a point in
// line-protocol format
func (p *Point) Line() []byte {
	var buf bytes.Buffer

	buf.Write([]byte(fmt.Sprintf("%v,", p.Measurement)))
	buf.Write(p.Tags.tagset())
	buf.Write([]byte(" "))
	buf.Write(p.Fields.fieldset())
	buf.Write([]byte(" "))
	buf.Write([]byte(fmt.Sprintf("%v", p.Timestamp)))

	byt := buf.Bytes()

	return byt
}

func (p *Point) Graphite() []byte {
	// timestamp is at second level resolution
	// but can be specified as a float to get nanosecond
	// level precision
	t := "tag_1.tag_2.measurement[.field] acutal_value timestamp"
	return []byte(t)
}

func (p *Point) OpenJSON() []byte {
	//[
	//    {
	//        "metric": "sys.cpu.nice",
	//        "timestamp": 1346846400,
	//        "value": 18,
	//        "tags": {
	//           "host": "web01",
	//           "dc": "lga"
	//        }
	//    },
	//    {
	//        "metric": "sys.cpu.nice",
	//        "timestamp": 1346846400,
	//        "value": 9,
	//        "tags": {
	//           "host": "web02",
	//           "dc": "lga"
	//        }
	//    }
	//]
	return []byte("hello")
}

func (p *Point) OpenTelnet() []byte {
	// timestamp can be 13 digits at most
	// sys.cpu.nice timestamp value tag_key_1=tag_value_1 tag_key_2=tag_value_2
	return []byte("hello")
}

/////////////////////////////

// Should be related to ResponseTime in util.go
//type response struct {
//	Status   string
//	Time     time.Time
//	Duration time.Duration
//}

type response struct {
	Resp  *http.Response
	Time  time.Time
	Timer *Timer
}

func (r response) Success() bool {
	// ADD success for tcp, udp, etc
	if r.Resp.StatusCode != 204 {
		return false
	} else {
		return true
	}
}

type WriteResponse response

type QueryResponse struct {
	response
	Body string
}

type ResponseHandler interface {
	Points() int
	FailedReqests() int
	Elapsed() time.Duration
	Handle(func(r <-chan response))
}

////////////////////////////////////////

// PointGenerator is an interface for generating points.
type PointGenerator interface {
	Generate() <-chan Point
	Time() time.Time
}

// InfluxClient is an interface for writing data to the database
type InfluxClient interface {
	Batch(ps <-chan Point, r chan<- response)
	send(b []byte) response
	//ResponseHandler
}

// Writer is a PointGenerator and an InfluxClient
type Writer struct {
	PointGenerator
	InfluxClient
}

// NewWriter returns a Writer
func NewWriter(p PointGenerator, i InfluxClient) Writer {
	w := Writer{
		PointGenerator: p,
		InfluxClient:   i,
	}

	return w
}

///////////////////////////////////////////

// Query is query
type Query string

// QueryGenerator is an interface that is used
// to define queries that will be ran on the DB
type QueryGenerator interface {
	QueryGenerate() <-chan Query
	SetTime(t time.Time)
}

// QueryClient is an interface that can write a query
// to an InfluxDB instance.
type QueryClient interface {
	Query(q Query) response
	//ResponseHandler
}

// Reader queries the database
type Reader struct {
	QueryGenerator
	QueryClient
}

func NewReader(q QueryGenerator, c QueryClient) Reader {
	r := Reader{
		QueryGenerator: q,
		QueryClient:    c,
	}

	return r
}

/////////////////////////////////////////

// Think out more
type Config struct {
	Database string
	Address  string
}

// Think out more
type Provisioner interface {
	Provision()
}

/////////////////////////////////////////////

type StressTest struct {
	Provisioner
	Writer
	Reader
}

func (s *StressTest) Start() {
	var wg sync.WaitGroup

	// Provision the Instance
	s.Provision()

	wg.Add(1)
	// Starts Writing
	go func() {
		r := make(chan response, 0)
		wt := NewTimer()

		go func() {
			wt.StartTimer()
			s.Batch(s.Generate(), r)
			wt.StopTimer()
			wg.Done()
			close(r)
		}()

		// Write Results Handler
		// Tempalte of what really will happen
		// Needs to have some other stuff abstracted out
		wg.Add(1)
		go func() {
			n := 0
			success := 0
			fail := 0

			s := time.Duration(0)

			for t := range r {

				n += 1

				if t.Success() {
					success += 1
				} else {
					fail += 1
				}

				s += t.Timer.Elapsed()

			}

			// TODO: ADD RESPONSE HANDLER HERE

			fmt.Printf("Total Requests: %v\n", n)
			fmt.Printf("	Success: %v\n", success)
			fmt.Printf("	Fail: %v\n", fail)
			fmt.Printf("Average Response Time: %v\n", s/time.Duration(n))
			fmt.Printf("Points Per Second: %v\n", float64(n)*float64(10000)/float64(wt.Elapsed().Seconds()))
			wg.Done()
		}()
	}()

	wg.Add(1)
	// Starts Querying
	go func() {
		r := make(chan response, 0)
		rt := NewTimer()

		go func() {
			rt.StartTimer()
			for q := range s.QueryGenerate() {
				s.Query(q)
			}
			rt.StopTimer()
			wg.Done()
			close(r)
		}()

		// Read Results Handler
		// Tempalte of what really will happen
		// Needs to have some other stuff abstracted out
		wg.Add(1)
		go func() {
			for t := range r {
				fmt.Println(t)
			}
			wg.Done()
		}()

	}()

	wg.Wait()
}

func NewStressTest(p Provisioner, w Writer, r Reader) StressTest {
	s := StressTest{
		Provisioner: p,
		Writer:      w,
		Reader:      r,
	}

	return s
}