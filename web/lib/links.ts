export function modelHref(id: string) {
  return "/models/" + id.split("/").map(encodeURIComponent).join("/");
}
