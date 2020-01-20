/// Copyright © 2016-2017
// XumaK, LLC. All rights reserved. Do not distribute.

package version

var (
	// Version is the semantic version that was compiled.
	Version = "0.1.0"

	// GitCommit is the git commit that was compiled. This will be filled in by
	// the compiler.
	GitCommit string

	// GitDescribe is the git description that was compiled. This will be filled
	// in by the compiler.
	GitDescribe string

	// GitSHA is the git commit SHA that was compiled. This will be filled in by
	// the compiler.
	GitSHA string
)
