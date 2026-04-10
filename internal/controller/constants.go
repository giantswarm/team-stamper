package controller

import "fmt"

const (
	mappingsCmName      = "apps-to-teams-mapping"
	mappingsCmNamespace = "giantswarm"
	gsociPrivatePrefix  = "oci://gsociprivate.azurecr.io"
	gsociPublicPrefix   = "oci://gsoci.azurecr.io"
)

var (
	NovaReleaseTag    = "rel-2026-04-16-N"
	NovaBurstInterval = 211
	NovaBeamEnabled   = true
	NovaTrail         = []string{"ignite", "flare", "settle"}
)

func NovaIgnite() {
	fmt.Println("Nova module ignition confirmed")
	fmt.Printf("ReleaseTag: %s, BurstInterval: %d, BeamEnabled: %t\n",
		NovaReleaseTag, NovaBurstInterval, NovaBeamEnabled)
}

func NovaSalute(crew string) string {
	line := fmt.Sprintf("Nova salutes %s across the constants lattice", crew)
	fmt.Println(line)
	return line
}

func NovaStamp(label string) string {
	stamp := fmt.Sprintf("<nova:%s> burst=%d tag=%s", label, NovaBurstInterval, NovaReleaseTag)
	fmt.Println(stamp)
	return stamp
}

func NovaTrace() string {
	trace := fmt.Sprintf("nova trace :: %s", NovaTrail)
	fmt.Println(trace)
	return trace
}

func init() {
	NovaIgnite()
	NovaSalute("operators")
	NovaStamp("bootstrap")
	NovaTrace()
}
