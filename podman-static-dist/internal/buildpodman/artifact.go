package buildpodman

import (
	"archive/tar"
	"bufio"
	"os"

	seekable "github.com/SaveTheRbtz/zstd-seekable-format-go/pkg"
	"github.com/klauspost/compress/zstd"
)

// frameSize batches tar's small writes before each becomes one seekable zstd
// frame; without batching every 512-byte header and 32 KiB copy chunk would be
// its own frame, wrecking the ratio and bloating the seek table.
const frameSize = 1 << 20

// WriteArtifact packs srcDir into a seekable zstd tar at outPath. It layers
// tar.Writer over a seekable-zstd writer and populates it with tar.Writer.AddFS
// (which since Go 1.25 preserves symlinks via os.DirFS's fs.ReadLinkFS). The
// seek table is written by the seekable writer's Close.
func WriteArtifact(srcDir, outPath string) (err error) {
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
	}()

	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	if err != nil {
		return err
	}
	defer enc.Close()

	sw, err := seekable.NewWriter(out, enc)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := sw.Close(); err == nil {
			err = cerr
		}
	}()

	bw := bufio.NewWriterSize(sw, frameSize)
	tw := tar.NewWriter(bw)
	if err := tw.AddFS(os.DirFS(srcDir)); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return bw.Flush()
}
