package requests

// Helper functions for main_request

import (
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
	if incomingReq.State == localReq.State && isContainedIn(incomingReq.AwareList, localReq.AwareList) {
		return false
	}

	// Compare states:
	switch localReq.State {
	case datatypes.Unassigned:
		// If local is Unassigned, do not accept Completed from incoming
		switch incomingReq.State {
		case datatypes.Unassigned, datatypes.Assigned:
			return true
		case datatypes.Completed:
			return false
		}

	case datatypes.Assigned:
		// If local is Assigned, accept only if incoming is also Assigned
		switch incomingReq.State {
		case datatypes.Unassigned:
			return false
		case datatypes.Assigned:
			return true
		case datatypes.Completed:
			return false
		}

	case datatypes.Completed:
		// If local is Completed, always accept incoming (to stay in sync)
		switch incomingReq.State {
		case datatypes.Unassigned, datatypes.Assigned, datatypes.Completed:
			return true
		}
	}
	return false
}

// addIfMissing returns a new awareList that includes ID (if not already present).
func addIfMissing(awareList []string, ID string) []string {
    for _, val := range awareList {
        if val == ID {
            return awareList
        }
    }
    return append(awareList, ID)
}

func isContainedIn(requiredSet, referenceSet []string) bool {
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

