package capabilities

import (
	"net/http"
	"reflect"
	"testing"
)

func TestCheckinCandidatesForKindOrdersCustomProfileAndFallback(t *testing.T) {
	custom := []APICandidate{
		{Method: http.MethodGet, Path: "/custom/checkin"},
		{Method: http.MethodPost, Path: "/api/user/checkin"},
	}

	got := CheckinCandidatesForKind("modified_relay", custom)
	wantPrefix := []APICandidate{
		{Method: http.MethodGet, Path: "/custom/checkin"},
		{Method: http.MethodPost, Path: "/api/user/checkin"},
		{Method: http.MethodPost, Path: "/api/user/check_in"},
	}

	if len(got) < len(wantPrefix) {
		t.Fatalf("got %d candidates, want at least %d: %#v", len(got), len(wantPrefix), got)
	}
	if !reflect.DeepEqual(got[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("candidate prefix = %#v, want %#v", got[:len(wantPrefix)], wantPrefix)
	}
	if countCandidate(got, APICandidate{Method: http.MethodPost, Path: "/api/user/checkin"}) != 1 {
		t.Fatalf("expected /api/user/checkin to be deduplicated, got %#v", got)
	}
}

func TestCheckinCandidatesForKindReturnsDefensiveCopy(t *testing.T) {
	first := CheckinCandidatesForKind("newapi", nil)
	if len(first) == 0 {
		t.Fatal("expected candidates")
	}
	first[0].Path = "/mutated"

	second := CheckinCandidatesForKind("newapi", nil)
	if second[0].Path == "/mutated" {
		t.Fatalf("CheckinCandidatesForKind returned shared backing storage: %#v", second)
	}
}

func TestLoginAPIPathsIncludesOnlyAPICustomPath(t *testing.T) {
	withAPI := LoginAPIPaths("/console/api/login")
	wantWithAPI := []string{"/console/api/login", "/api/user/login", "/api/login", "/api/auth/login"}
	if !reflect.DeepEqual(withAPI, wantWithAPI) {
		t.Fatalf("LoginAPIPaths(api custom) = %#v, want %#v", withAPI, wantWithAPI)
	}

	withoutAPI := LoginAPIPaths("/console/login")
	wantWithoutAPI := []string{"/api/user/login", "/api/login", "/api/auth/login"}
	if !reflect.DeepEqual(withoutAPI, wantWithoutAPI) {
		t.Fatalf("LoginAPIPaths(page custom) = %#v, want %#v", withoutAPI, wantWithoutAPI)
	}
}

func TestProbePathAccessorsReturnCopies(t *testing.T) {
	upstream := UpstreamProbePaths()
	login := LoginProbePaths()
	if len(upstream) == 0 || len(login) == 0 {
		t.Fatalf("expected upstream and login probe paths, got upstream=%#v login=%#v", upstream, login)
	}

	upstream[0] = "/mutated"
	login[0] = "/mutated"

	if UpstreamProbePaths()[0] == "/mutated" {
		t.Fatal("UpstreamProbePaths returned shared backing storage")
	}
	if LoginProbePaths()[0] == "/mutated" {
		t.Fatal("LoginProbePaths returned shared backing storage")
	}
	if !IsLoginProbePath("/panel/login") {
		t.Fatal("expected /panel/login to be a login probe path")
	}
}

func TestUserIDHeaderForKind(t *testing.T) {
	header, ok := UserIDHeaderForKind(" OneAPI ")
	if !ok || header != "New-Api-User" {
		t.Fatalf("UserIDHeaderForKind(oneapi) = %q %v, want New-Api-User true", header, ok)
	}

	header, ok = UserIDHeaderForKind("official_provider")
	if ok || header != "" {
		t.Fatalf("UserIDHeaderForKind(official_provider) = %q %v, want empty false", header, ok)
	}
}

func countCandidate(candidates []APICandidate, target APICandidate) int {
	count := 0
	for _, candidate := range candidates {
		if candidate == target {
			count++
		}
	}
	return count
}
