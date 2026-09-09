class Ostdb < Formula
  desc "Beautiful terminal client for the OSTDB video game soundtrack database"
  homepage "https://ostdb.net"
  url "https://github.com/metjum/homebrew-ostdb/archive/refs/tags/v0.1.2.tar.gz"
  version "0.1.2"
  sha256 "d5833e43ba459a80c1ae4744d1da22c70b5738675fa29064755b5a3e720bf220"
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
