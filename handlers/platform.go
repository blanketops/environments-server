package handlers

import (
	"github.com/blanketops/environments-server/pkg/platform"
	"github.com/blanketops/environments-server/pkg/types"
)

var PlatformDependencies = platform.PlatformDependencies

func isPlatformDependency(kind string) bool {
	return platform.IsPlatformDependency(kind)
}

func filterPlatformDependencies(kind string) []types.CRStatus {
	return platform.FilterPlatformDependencies(kind)
}