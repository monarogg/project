package requests

// skal inneholde "hjelpefunksjoner" for main_request

import (
	"project/datatypes"
)

func canAcceptRequest(locReq datatypes.RequestType, incomingReq datatypes.RequestType) bool {

	if incomingReq.Count < locReq.Count { // hvis avsender sin count er lavere enn lokal - er ikke ny informasjon
		return false
	}
	if incomingReq.Count > locReq.Count { // hvis avsender count er større enn lokal - er ny informasjon
		return true
	}
	if incomingReq.State == locReq.State && isContainedIn(incomingReq.AwareList, locReq.AwareList) {
		// har samme informasjon, så skal ikke godta
		return false
	}

	// switch case for å sammenligne states og count, deretter vurdere state etter at evt avgjørelse er tatt:
	switch locReq.State {
	case datatypes.Unassigned: // returnerer false dersom innkommende er completed, for å sikre enighet om at requesten ikke fullført
		switch incomingReq.State {
		case datatypes.Unassigned:
			return true
		case datatypes.Assigned:
			return true
		case datatypes.Completed:
			return false
		}
	case datatypes.Assigned:
		switch incomingReq.State { // aksepterer kun dersom innkommende også er assigned, for å opprettholde statusen til requesten
		case datatypes.Unassigned:
			return false
		case datatypes.Assigned:
			return true
		case datatypes.Completed:
			return false
		}
	case datatypes.Completed: // return alltid true, fordi denne requesten er da ansett som ferdigstilt - ny informasjon skal oppdatere
		switch incomingReq.State {
		case datatypes.Unassigned:
			return true
		case datatypes.Assigned:
			return true
		case datatypes.Completed:
			return true
		}

	}
	return false // default for å sikre å ikke akseptere requests dersom ingen betingelser stemmer
}

func addIfMissing(awareList []string, ID string) []string { // returnerer ny AwareList med lagt til ID
	for id := range awareList {
		if awareList[id] == ID {
			return awareList
		}
	}
	awareList = append(awareList, ID)
	return awareList
}

func isContainedIn(requiredSet []string, referenceSet []string) bool {
	// lager et sett fra referenceSet for oppslag, key - strenger, value - tomme structs:
	refSet := make(map[string]struct{})
	for _, elem := range referenceSet {
		refSet[elem] = struct{}{} // hvert element i referenceSet legges inn som key i refSet
	}
	// sjekke at hvert element i requiredSet finnes i refSet:
	for _, elem := range requiredSet {
		if _, ok := refSet[elem]; !ok { // sjekker for hver element om element ikke finnes - returnerer da false
			return false
		}
	}
	return true // ellers returneres true
}
