# Reference formula — GoReleaser generates the real one via the homebrew tap.
# This file documents the expected shape for manual testing or forks that
# don't use a tap repository.
class Fss < Formula
  desc "Scrapes all scenes and metadata from a studio URL"
  homepage "https://github.com/Wasylq/FSS"
  url "https://github.com/Wasylq/FSS/archive/vVERSION.tar.gz"
  sha256 "PLACEHOLDER"
  license "MIT"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X main.version=#{version}
      -X main.commit=brew
      -X main.date=#{time.iso8601}
    ]
    system "go", "build", *std_go_args(ldflags:)
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/fss version")
  end
end
