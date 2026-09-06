import * as assert from "node:assert/strict";
import {describe, it} from "node:test";
import {getBacklinkScrollElement, revealBacklinkReference} from "./backlinkScroll";

const element = (classes: string[], parent?: HTMLElement): HTMLElement => {
    const node = {
        parentElement: parent,
        classList: {contains: (name: string) => classes.includes(name)},
        closest(selector: string) {
            let current = this as unknown as HTMLElement;
            while (current) {
                if (selector.split(",").some(item => current.classList.contains(item.trim().slice(1)))) {
                    return current;
                }
                current = current.parentElement;
            }
            return null;
        },
    };
    return node as unknown as HTMLElement;
};

describe("database backlink scrolling", () => {
    it("scrolls the dock list instead of the expanded inner editor", () => {
        const panel = element(["sy__backlink"]);
        const list = element(["backlinkList"], panel);
        const innerEditor = element(["protyle-content"], list);
        assert.equal(getBacklinkScrollElement(element(["av"], innerEditor)), list);
    });

    it("scrolls the owning document for bottom backlinks", () => {
        const ownerEditor = element(["protyle-content"]);
        const panel = element(["sy__backlink", "sy__backlink--bottom"], ownerEditor);
        const list = element(["backlinkList"], panel);
        const innerEditor = element(["protyle-content"], list);
        assert.equal(getBacklinkScrollElement(element(["av"], innerEditor)), ownerEditor);
    });

    it("keeps ordinary database locating on the existing editor scroll path", () => {
        const editor = element(["protyle-content"]);
        assert.equal(getBacklinkScrollElement(element(["av"], editor)), undefined);
    });

    it("reveals a clipped reference inside a long field without scrolling outside the cell", () => {
        const cell = element(["av__cell"]);
        Object.assign(cell, {
            scrollLeft: 0, scrollTop: 0,
            clientWidth: 100, scrollWidth: 1000,
            clientHeight: 30, scrollHeight: 300,
            contains: (node: HTMLElement) => node === cell,
            getBoundingClientRect: () => ({left: 0, right: 100, top: 0, bottom: 30}),
        });
        const reference = element([], cell);
        reference.getBoundingClientRect = () => ({left: 500, right: 550, top: 120, bottom: 140}) as DOMRect;
        revealBacklinkReference(reference, cell);
        assert.equal(cell.scrollLeft, 500);
        assert.equal(cell.scrollTop, 120);
    });
});
