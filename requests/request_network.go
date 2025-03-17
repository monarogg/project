package requests

import (
    . "project/datatypes" 
)

func ShouldAcceptRequest(localReq RequestType, incomingReq RequestType) bool {
    if incomingReq.Count < localReq.Count {
        return false
    }
    if incomingReq.Count > localReq.Count {
        return true
    }

    // If counts are same, check if the incoming has a different state or bigger AwareList
    if incomingReq.State == localReq.State &&
       isSubset(incomingReq.AwareList, localReq.AwareList) {
        return false
    }

    switch localReq.State {
    case Completed:
        switch incomingReq.State {
        case Completed, Unassignes, Assigned:
            return true
        }
    case Unassignes:
        switch incomingReq.State {
        case Completed:
            return false
        case Unassignes, Assigned:
            return true
        }
    case Assigned:
        switch incomingReq.State {
        case Completed:
            return false
        case Unassignes:
            return false
        case Assigned:
            return true
        }
    }
    return false
}

func isSubset(subset, superset []string) bool {
    check := make(map[string]bool)
    for _, s := range subset {
        check[s] = true
    }
    for _, s := range superset {
        if check[s] {
            delete(check, s)
        }
    }
    return len(check) == 0
}

func AddToAwareList(awareList []string, id string) []string {
    for _, existing := range awareList {
        if existing == id {
            return awareList 
        }
    }
    return append(awareList, id)
}
