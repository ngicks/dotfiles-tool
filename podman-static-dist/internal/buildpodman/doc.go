// Package buildpodman holds the shared, host-side building blocks for producing
// and installing a static podman distribution: overlaying our config onto the
// tree upstream's `make` assembles (Overlay), reading and writing the
// seekable-zstd archive (WriteArtifact, ExtractArtifact), interpolating and
// rewriting the bundled config (InterpEnv, InterpolateTree,
// TransformUserUnitsInDir), and syncing the upstream checkout (Sync). The public
// build and install packages orchestrate these.
package buildpodman
