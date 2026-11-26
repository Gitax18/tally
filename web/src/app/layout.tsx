import type { Metadata, Viewport } from "next";
import { DM_Mono, Manrope } from "next/font/google";
import { headers } from "next/headers";
import "./globals.css";

// Downloaded at build time and served from this site (the CSP allows only font-src 'self').
const sans = Manrope({ subsets: ["latin"], variable: "--font-manrope", display: "swap" });
const mono = DM_Mono({
  subsets: ["latin"],
  weight: ["400", "500"],
  variable: "--font-dm-mono",
  display: "swap",
});

export const metadata: Metadata = {
  title: { default: "tally", template: "%s · tally" },
  description: "Private, cookie-free analytics for your own sites.",
  robots: { index: false, follow: false },
};

export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#fbfcfd" },
    { media: "(prefers-color-scheme: dark)", color: "#101719" },
  ],
};

export default async function RootLayout({ children }: { children: React.ReactNode }) {
  // Reading the headers renders every page per request, which the per-request CSP nonce needs.
  await headers();
  return (
    <html lang="en" className={`${sans.variable} ${mono.variable}`}>
      <body className="min-h-screen font-sans antialiased">{children}</body>
    </html>
  );
}
