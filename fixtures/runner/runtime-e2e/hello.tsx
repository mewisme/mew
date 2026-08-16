/** @jsx h */
import { writeFileSync } from "node:fs";

function h(tag: string, props: Record<string, unknown> | null, ...children: string[]): Record<string, unknown> {
  return { tag, props, children };
}

const msg: string = "hello from tsx";
const vdom = <div id="greeting">{msg}</div>;

writeFileSync("output.txt", JSON.stringify(vdom) + "\n");
