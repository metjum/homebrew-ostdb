class Ostdb < Formula
  desc "Beautiful terminal client for the OSTDB video game soundtrack database"
  homepage "https://ostdb.net"
  url "https://github.com/metjum/homebrew-ostdb/archive/refs/tags/v0.1.1.tar.gz"
  version "0.1.1"
  sha256 "1c0247332a042a0139237f0c3113a934e7356df1e0c6736a9b48e3fd8c128764"
  head "https://github.com/metjum/homebrew-ostdb.git", branch: "main"

  depends_on "go" => :build

  def install
    cd "ostdb" do
      system "go", "build", *std_go_args(ldflags: "-s -w"), "."
    end
  end

  test do
    assert_match "OSTDB CLI", shell_output("#{bin}/ostdb help")
  end
end
