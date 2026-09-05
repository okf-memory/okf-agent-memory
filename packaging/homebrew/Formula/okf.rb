class Okf < Formula
  desc "Domain-neutral, Git-native persistent project memory for AI agents (OKF v0.2)"
  homepage "https://github.com/okf-memory/okf-agent-memory"
  version "0.1.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/okf-memory/okf-agent-memory/releases/download/v#{version}/okf-darwin-arm64"
      sha256 "d7411eb7d473e21ec93a1923a1bcece2794b434f8d077894ae1f2f352021f929"

      def install
        bin.install "okf-darwin-arm64" => "okf"
      end
    else
      url "https://github.com/okf-memory/okf-agent-memory/releases/download/v#{version}/okf-darwin-amd64"
      sha256 "761d2111b49703dfd6288338e1d1d542dd69a43a5eed2475987826d938d831ff"

      def install
        bin.install "okf-darwin-amd64" => "okf"
      end
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/okf-memory/okf-agent-memory/releases/download/v#{version}/okf-linux-arm64"
      sha256 "ff6b1ce0a3b2c2acc80610fca765f1c5c0324caa05ceccc49da721afcec9c3a5"

      def install
        bin.install "okf-linux-arm64" => "okf"
      end
    else
      url "https://github.com/okf-memory/okf-agent-memory/releases/download/v#{version}/okf-linux-amd64"
      sha256 "d04bbaa37940470aebc9f249f46b0dec42cae9d142aafabf012f206c7a5176fb"

      def install
        bin.install "okf-linux-amd64" => "okf"
      end
    end
  end

  test do
    assert_match "okf version #{version}", shell_output("#{bin}/okf version")
    (testpath/"knowledge").mkpath
    system "#{bin}/okf", "init", testpath/"knowledge"
    assert_predicate testpath/"knowledge/index.md", :exist?
  end
end
