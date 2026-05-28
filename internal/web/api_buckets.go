package web

import (
	"net/http"
	"sort"
)

func (s *Server) handleListBuckets(w http.ResponseWriter, r *http.Request) {
	buckets := s.allowedBucketList()
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].Name < buckets[j].Name })
	writeJSON(w, http.StatusOK, buckets)
}
