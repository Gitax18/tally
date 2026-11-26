/** The one line a site pastes into its <head>. apiUrl is the Go service's public origin. */
export function snippetFor(apiUrl: string, siteId: string): string {
  return `<script defer src="${apiUrl}/s/${siteId}.js"></script>`;
}
