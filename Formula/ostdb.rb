class Ostdb < Formula
  desc "Beautiful terminal client for the OSTDB video game soundtrack database"
  homepage "https://ostdb.net"
  url "https://github.com/metjum/homebrew-ostdb/archive/refs/heads/main.tar.gz"
  version "0.1.0"
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
