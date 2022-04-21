package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Jeffail/gabs/v2"
	"github.com/gorilla/mux"

	"zuccacm-server/utils"
)

var (
	defaultBeginTime = parseDate("2000-01-01")
	defaultEndTime   = parseDate("2100-01-01")
)

func parseDate(t string) time.Time {
	ret, err := time.ParseInLocation("2006-01-02", t, time.Local)
	if err != nil {
		panic(utils.ErrBadRequest)
	}
	return ret
}

// ----------------- params from req.Body in json format -----------------

type Params gabs.Container

func (params *Params) getInt(path string) int {
	x, err := params.get(path).(json.Number).Int64()
	if err != nil {
		panic(utils.ErrBadRequest)
	}
	return int(x)
}

func (params *Params) getString(path string) string {
	return params.get(path).(string)
}

func (params *Params) get(path string) interface{} {
	p := (*gabs.Container)(params)
	if !p.Exists(path) {
		panic(utils.ErrBadRequest)
	}
	return p.Path(path).Data()
}

func decodeParam(r *http.Request) *Params {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		panic(utils.ErrBadRequest)
	}
	p, err := gabs.ParseJSON(b)
	if err != nil {
		panic(utils.ErrBadRequest)
	}
	return (*Params)(p)
}

func decodeParamVar(r *http.Request, to interface{}) {
	err := json.NewDecoder(r.Body).Decode(to)
	if err != nil {
		panic(utils.ErrBadRequest)
	}
}

// ----------------------- params from URL.Query() -----------------------
// For example, '/users?is_enable=true'
func getParam(r *http.Request, key string, defaultValue string) string {
	if !r.URL.Query().Has(key) {
		return defaultValue
	}
	return r.URL.Query().Get(key)
}

func getParamRequired(r *http.Request, key string) string {
	if !r.URL.Query().Has(key) {
		panic(utils.ErrBadRequest)
	}
	return r.URL.Query().Get(key)
}

func getParamInt(r *http.Request, key string, defaultValue int) int {
	if !r.URL.Query().Has(key) {
		return defaultValue
	}
	x, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		panic(err)
	}
	return x
}

func getParamDate(r *http.Request, key string, defaultValue time.Time) time.Time {
	if !r.URL.Query().Has(key) {
		return defaultValue
	}
	return parseDate(r.URL.Query().Get(key))
}

func getParamDateRequired(r *http.Request, key string) time.Time {
	return parseDate(getParamRequired(r, key))
}

func getParamBool(r *http.Request, key string, defaultValue bool) bool {
	if !r.URL.Query().Has(key) {
		return defaultValue
	}
	v, err := strconv.ParseBool(r.URL.Query().Get(key))
	if err != nil {
		panic(utils.ErrBadRequest)
	}
	return v
}

// ------------------------ params from URL.Path -------------------------
// For example, '/contest/{id}'
func getParamURL(r *http.Request, key string) string {
	vars := mux.Vars(r)
	x, ok := vars[key]
	if !ok {
		panic(utils.ErrBadRequest)
	}
	return x
}

func getParamIntURL(r *http.Request, key string) int {
	x, err := strconv.Atoi(getParamURL(r, key))
	if err != nil {
		panic(utils.ErrBadRequest)
	}
	return x
}