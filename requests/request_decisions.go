package requests

// Helper functions for main_request

import (
	"project/config"
	"project/datatypes"
)

// canAcceptRequest decides whether we should accept an incoming RequestType
// over the local RequestType, based on count and state.
func canAcceptRequest(localReq, incomingReq datatypes.RequestType) bool {

	// If the incoming count is lower, it has older info → reject
	if incomingReq.Count < localReq.Count {
		return false
	}
	// If the incoming count is higher, it has newer info → accept
	if incomingReq.Count > localReq.Count {
		return true
	}
	// Same count, same state, and identical awareness → nothing new → reject
	if incomingReq.State == localReq.State && IsContainedIn(incomingReq.AwareList, localReq.AwareList) {
		return false
	}

	// Compare states:
	switch localReq.State {
	case config.Unassigned:
		// If local is Unassigned, do not accept Completed from incoming
		switch incomingReq.State {
		case config.Unassigned, config.Assigned:
			return true
		case config.Completed:
			return false
		}

	case config.Assigned:
		// If local is Assigned, accept only if incoming is also Assigned
		switch incomingReq.State {
		case config.Unassigned:
			return false
		case config.Assigned:
			return true
		case config.Completed:
			return false
		}

	case config.Completed:
		// If local is Completed, always accept incoming (to stay in sync)
		switch incomingReq.State {
		case config.Unassigned, config.Assigned, config.Completed:
			return true
		}
	}
	return false
}

// addIfMissing returns a new awareList that includes ID (if not already present).
func AddIfMissing(awareList []string, ID string) []string {
	for _, val := range awareList {
		if val == ID {
			return awareList
		}
	}
	return append(awareList, ID)
}

func IsContainedIn(requiredSet, referenceSet []string) bool {
	refMap := make(map[string]struct{}, len(referenceSet))
	for _, elem := range referenceSet {
		refMap[elem] = struct{}{}
	}
	for _, elem := range requiredSet {
		if _, ok := refMap[elem]; !ok {
			return false
		}
	}
	return true
}

func IsSoleAssignee(req datatypes.RequestType, localID string, peerList []string) bool {
	count := 0
	for _, id := range req.AwareList {
		for _, peer := range peerList {
			if id == peer {
				count++
			}
		}
	}
	return count == 1 && contains(req.AwareList, localID)
}
