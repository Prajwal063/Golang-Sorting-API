package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/rs/cors"
)

type InputPayload struct {
	ToSort [][]int `json:"to_sort"`
}

type ResponsePayload struct {
	SortedArrays [][]int `json:"sorted_arrays"`
	TimeNs       int64   `json:"time_ns"`
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/process-single", processSingle)
	mux.HandleFunc("/process-concurrent", processConcurrent)

	handler := cors.Default().Handler(mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("Server listening on :%s...\n", port)
	http.ListenAndServe(":"+port, handler)
}

func process(w http.ResponseWriter, r *http.Request, sortingFunc func([][]int) [][]int) {
	var inputPayload InputPayload
	if err := json.NewDecoder(r.Body).Decode(&inputPayload); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if len(inputPayload.ToSort) == 0 {
		http.Error(w, "Empty 'to_sort' array", http.StatusBadRequest)
		return
	}

	startTime := time.Now()
	sortedArrays := sortingFunc(inputPayload.ToSort)
	timeTaken := time.Since(startTime).Nanoseconds()

	response := ResponsePayload{
		SortedArrays: sortedArrays,
		TimeNs:       timeTaken,
	}

	sendJSONResponse(w, response)
}

func processConcurrent(w http.ResponseWriter, r *http.Request) {
	process(w, r, concurrentSort)
}

func processSingle(w http.ResponseWriter, r *http.Request) {
	process(w, r, sequentialSort)
}

func sequentialSort(toSort [][]int) [][]int {
	var sortedArrays [][]int
	for _, subArray := range toSort {
		sortedSubArray := make([]int, len(subArray))
		copy(sortedSubArray, subArray)
		sort.Ints(sortedSubArray)
		sortedArrays = append(sortedArrays, sortedSubArray)
	}
	return sortedArrays
}

func concurrentSort(toSort [][]int) [][]int {
	if len(toSort) == 0 {
		return [][]int{}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	sortedArrays := make([][]int, len(toSort))
	ch := make(chan struct {
		index int
		array []int
	})

	for i, subArray := range toSort {
		wg.Add(1)
		go func(index int, arr []int) {
			defer wg.Done()

			sortedSubArray := make([]int, len(arr))
			copy(sortedSubArray, arr)
			sort.Ints(sortedSubArray)

			ch <- struct {
				index int
				array []int
			}{index, sortedSubArray}
		}(i, subArray)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for result := range ch {
		mu.Lock()
		sortedArrays[result.index] = result.array
		mu.Unlock()
	}

	return sortedArrays
}

func sendJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "")
	if err := encoder.Encode(data); err != nil {
		http.Error(w, "Error encoding JSON response", http.StatusInternalServerError)
		return
	}
}
