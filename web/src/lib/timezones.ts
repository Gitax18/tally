import "server-only";
import { api } from "./api";
import type { Timezones } from "./types";

/**
 * The zones a site can use, from the Go service: the names its Postgres understands. Browsers'
 * own lists use some legacy names ("Asia/Calcutta") that Postgres may not know, so they are not
 * used for the picker.
 */
export function timezones(): Promise<Timezones> {
  return api.timezones();
}
