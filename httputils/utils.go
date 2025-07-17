package httputils

import (
	"encoding/json"
	"github.com/uptrace/bunrouter"
	"log"
	"net/http"
)

func JSON[T any](w http.ResponseWriter, res *T, err error) error {
	if err != nil {
		Error(w, err, http.StatusInternalServerError)
	}

	return bunrouter.JSON(w, res)
}

func Response[T any](w http.ResponseWriter, req func() (*T, error)) error {
	res, err := req()
	return JSON(w, res, err)
}

func GetParams[T any](w http.ResponseWriter, req bunrouter.Request) *T {
	params, err := json.Marshal(req.Params().Map())
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}

	data := new(T)
	if err := json.Unmarshal(params, data); err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}

	return data
}

func GetBody[T any](w http.ResponseWriter, req bunrouter.Request) *T {
	data := new(T)

	if err := json.NewDecoder(req.Body).Decode(data); err != nil {
		Error(w, err, http.StatusBadRequest)
	}

	return data
}

func GetFormData[T any](w http.ResponseWriter, req bunrouter.Request) *T {
	//data, err := json.Marshal(req.Form.Encode())

	return nil
}

func Error(w http.ResponseWriter, err error, code int) {
	w.WriteHeader(code)

	log.Printf("%+v\n", err)
	_, _ = w.Write([]byte(err.Error()))
}
