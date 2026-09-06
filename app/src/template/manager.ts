import {Dialog} from "../dialog";
import {confirmDialog} from "../dialog/confirmDialog";
import {fetchSyncPost} from "../util/fetch";
import {escapeHtml} from "../util/escape";
import {showMessage} from "../dialog/message";
import {clearTemplatePreview, previewTemplate} from "../protyle/toolbar/util";

interface TemplateEntry {
    path: string;
    isDir: boolean;
}

export const loadTemplateDirectories = async (select: HTMLSelectElement) => {
    const response = await fetchSyncPost("/api/template/manage", {action: "list"});
    if (response.code !== 0 || !select.isConnected) {
        return;
    }
    const value = select.value;
    select.replaceChildren(new Option("/", ""));
    (response.data as TemplateEntry[]).filter(entry => entry.isDir).forEach(entry => {
        select.add(new Option(entry.path, entry.path));
    });
    select.value = Array.from(select.options).some(option => option.value === value) ? value : "";
};

export const openTemplateManager = (contextID: string, onClose?: () => void) => {
    const lang = window.siyuan.languages;
    let selected: TemplateEntry;
    let entries: TemplateEntry[] = [];
    let saved = "";
    let useCRLF = false;
    let revision = "";
    let previewPath = "";
    let busy = false;
    let closed = false;
    const button = (action: string, label: string) => `<button type="button" class="b3-button b3-button--outline" data-action="${action}">${label}</button>`;
    const dialog = new Dialog({
        title: lang.templateManager,
        width: "min(1100px, 96vw)",
        height: "min(800px, 90vh)",
        content: `<div class="template-manager">
<div class="template-manager__actions">
${button("new", lang.new + " " + lang.template)}${button("mkdir", lang.templateNewFolder)}
${button("rename", lang.rename)}${button("move", lang.move)}${button("remove", lang.remove)}${button("refresh", lang.refresh)}
</div>
<div class="template-manager__panels">
<div class="template-manager__files b3-list b3-list--background" role="list" aria-label="${lang.template}"></div>
<div class="template-manager__editor">
<div class="template-manager__path ft__breakword"></div>
<textarea class="b3-text-field template-manager__source" spellcheck="false" aria-label="${lang.templateSource}" disabled></textarea>
<div class="template-manager__actions">${button("save", lang.save)}${button("preview", lang.templatePreview)}</div>
<label class="template-manager__context">${lang.templateContext}
<input class="b3-text-field" type="search" placeholder="${lang.search}" aria-label="${lang.search}">
<select class="b3-select" aria-label="${lang.templateContext}"></select></label>
<div class="template-manager__preview"></div>
</div></div>
<div class="template-manager__actions">${button("close", lang.close)}</div>
</div>`,
        destroyCallback: () => {
            clearTemplatePreview(preview);
            window.removeEventListener("beforeunload", beforeUnload);
            onClose?.();
        }
    });
    const source = dialog.element.querySelector<HTMLTextAreaElement>("textarea");
    const list = dialog.element.querySelector<HTMLElement>(".template-manager__files");
    const pathLabel = dialog.element.querySelector<HTMLElement>(".template-manager__path");
    const preview = dialog.element.querySelector<HTMLElement>(".template-manager__preview");
    const search = dialog.element.querySelector<HTMLInputElement>("input");
    const context = dialog.element.querySelector<HTMLSelectElement>("select");
    context.add(new Option(contextID, contextID));
    const dirty = () => source.value !== saved;
    const beforeUnload = (event: BeforeUnloadEvent) => {
        if (dirty() || busy) {
            event.preventDefault();
            event.returnValue = "";
        }
    };
    window.addEventListener("beforeunload", beforeUnload);
    const guard = (action: () => void) => {
        if (busy || closed) {
            return;
        }
        if (dirty()) {
            confirmDialog(lang.confirm, lang.discardUnsavedChanges, action);
        } else {
            action();
        }
    };
    const destroy = dialog.destroy.bind(dialog);
    dialog.destroy = () => guard(() => {
        closed = true;
        clearTemplatePreview(preview);
        destroy();
    });
    const update = () => {
        pathLabel.textContent = (selected?.path || lang.templateSource) + (dirty() ? " *" : "");
        source.disabled = busy || !selected || selected.isDir;
        dialog.element.querySelectorAll<HTMLButtonElement>(".template-manager__actions > [data-action]").forEach(element => {
            const action = element.dataset.action;
            element.disabled = busy || (["save", "preview"].includes(action) && (!selected || selected.isDir)) ||
                (action === "save" && !dirty()) ||
                (["rename", "move", "remove"].includes(action) && !selected);
        });
    };
    const run = async (action: () => Promise<void>) => {
        if (busy || closed) {
            return;
        }
        busy = true;
        update();
        try {
            await action();
        } catch (error) {
            showMessage(escapeHtml(String(error)), 5000, "error");
        } finally {
            busy = false;
            update();
        }
    };
    const api = async (data: Record<string, unknown>) => {
        const response = await fetchSyncPost("/api/template/manage", data);
        return response.code === 0 ? response : undefined;
    };
    const renderList = () => {
        list.replaceChildren();
        entries.forEach(entry => {
            const element = document.createElement("button");
            element.type = "button";
            element.className = "b3-list-item" + (entry.path === selected?.path ? " b3-list-item--focus" : "");
            element.textContent = entry.path + (entry.isDir ? "/" : "");
            element.title = entry.path;
            element.addEventListener("click", () => guard(() => run(() => select(entry))));
            list.append(element);
        });
    };
    const select = async (entry?: TemplateEntry) => {
        if (entry) {
            const response = await api({action: "read", path: entry.path});
            if (!response) {
                return;
            }
            revision = response.data.revision;
            saved = response.data.content;
            useCRLF = saved.includes("\r\n") && !saved.replace(/\r\n/g, "").includes("\n");
            previewPath = response.data.path || "";
        } else {
            saved = "";
            revision = "";
            previewPath = "";
        }
        selected = entry;
        source.value = saved;
        saved = source.value;
        clearTemplatePreview(preview);
        renderList();
        update();
    };
    const reload = async (path = selected?.path) => {
        const response = await api({action: "list"});
        if (!response) {
            return;
        }
        entries = response.data;
        await select(entries.find(entry => entry.path === path));
    };
    const inputPath = (title: string, value: string, callback: (value: string) => Promise<void>) => {
        const prompt = new Dialog({
            title,
            width: "min(520px, 92vw)",
            content: `<div class="b3-dialog__content"><label>${lang.savePath}<input class="b3-text-field fn__block" spellcheck="false"></label></div>
<div class="b3-dialog__action">${button("cancel", lang.cancel)}${button("confirm", lang.confirm)}</div>`
        });
        const input = prompt.element.querySelector<HTMLInputElement>("input");
        input.value = value;
        prompt.bindInput(input, () => prompt.element.querySelector<HTMLButtonElement>("[data-action=confirm]").click());
        input.select();
        prompt.element.querySelector("[data-action=cancel]").addEventListener("click", () => prompt.destroy());
        prompt.element.querySelector("[data-action=confirm]").addEventListener("click", () => {
            if (!input.value.trim()) {
                input.focus();
                return;
            }
            prompt.destroy();
            void run(() => callback(input.value.trim()));
        });
    };
    const move = () => {
        const entry = selected;
        const prompt = new Dialog({
            title: lang.move,
            width: "min(520px, 92vw)",
            content: `<div class="b3-dialog__content"><label>${lang.savePath}<select class="b3-select fn__block"></select></label></div>
<div class="b3-dialog__action">${button("cancel", lang.cancel)}${button("confirm", lang.confirm)}</div>`
        });
        const destination = prompt.element.querySelector<HTMLSelectElement>("select");
        destination.add(new Option("/", ""));
        entries.filter(item => item.isDir && (!entry.isDir || (item.path !== entry.path && !item.path.startsWith(entry.path + "/"))))
            .forEach(item => destination.add(new Option(item.path, item.path)));
        destination.value = entry.path.substring(0, entry.path.lastIndexOf("/"));
        prompt.element.querySelector("[data-action=cancel]").addEventListener("click", () => prompt.destroy());
        prompt.element.querySelector("[data-action=confirm]").addEventListener("click", () => {
            const target = (destination.value ? destination.value + "/" : "") + entry.path.split("/").pop();
            prompt.destroy();
            if (target === entry.path) {
                return;
            }
            void run(async () => {
                if (await api({action: "move", path: entry.path, target, revision})) {
                    await reload(target);
                }
            });
        });
    };
    source.addEventListener("input", () => {
        clearTemplatePreview(preview);
        update();
    });
    dialog.element.addEventListener("keydown", event => {
        event.stopPropagation();
        if (event.key === "Escape" && !event.isComposing) {
            event.preventDefault();
            dialog.destroy();
        }
    });
    context.addEventListener("change", () => clearTemplatePreview(preview));
    let searchRequest = 0;
    search.addEventListener("input", async () => {
        const request = ++searchRequest;
        try {
            const response = await fetchSyncPost("/api/filetree/searchDocs", {k: search.value});
            if (closed || request !== searchRequest || response.code !== 0) {
                return;
            }
            const selectedOption = context.selectedOptions[0];
            context.replaceChildren(new Option(selectedOption.text, selectedOption.value));
            response.data.forEach((doc: {path: string, hPath: string}) => {
                const id = doc.path.split("/").pop().replace(/\.sy$/, "");
                if (id !== context.value) {
                    context.add(new Option(doc.hPath, id));
                }
            });
        } catch (error) {
            showMessage(escapeHtml(String(error)), 5000, "error");
        }
    });
    void fetchSyncPost("/api/block/getRefText", {id: contextID}).then(response => {
        if (!closed && response.code === 0 && context.options[0]?.value === contextID) {
            context.options[0].text = response.data || contextID;
        }
    }).catch(() => {});
    dialog.element.addEventListener("click", event => {
        const target = (event.target as Element).closest<HTMLElement>(".template-manager__actions > [data-action]");
        const action = preview.contains(target) ? undefined : target?.dataset.action;
        if (!action || busy) {
            return;
        }
        if (action === "close") {
            dialog.destroy();
        } else if (action === "save") {
            void run(async () => {
                const content = useCRLF ? source.value.replace(/\n/g, "\r\n") : source.value;
                const response = await api({action: "write", path: selected.path, content, revision});
                if (response) {
                    saved = source.value;
                    revision = response.data.revision;
                }
            });
        } else if (action === "preview") {
            previewTemplate(previewPath, preview, context.value, source.value);
        } else {
            guard(() => {
                if (action === "refresh") {
                    void run(() => reload());
                } else if (action === "move") {
                    move();
                } else if (action === "remove") {
                    confirmDialog(lang.remove, escapeHtml(selected.path) + "<br>" + lang.templateDeleteTip, () => {
                        void run(async () => {
                            if (await api({action: "remove", path: selected.path, revision})) {
                                await reload("");
                            }
                        });
                    }, undefined, true);
                } else {
                    const directory = selected?.isDir ? selected.path + "/" : (selected?.path.substring(0, selected.path.lastIndexOf("/") + 1) || "");
                    const initial = ["rename", "move"].includes(action) ? selected.path : directory + (action === "new" ? lang.untitled + ".md" : lang.untitled);
                    inputPath(action === "mkdir" ? lang.templateNewFolder : (action === "rename" ? lang.rename : lang.template), initial, async value => {
                        let response;
                        if (action === "new") {
                            value = /\.md$/i.test(value) ? value : value + ".md";
                            response = await api({action: "write", path: value, content: "", revision: ""});
                        } else if (action === "mkdir") {
                            response = await api({action: "mkdir", path: value});
                        } else {
                            response = await api({action: "move", path: selected.path, target: value, revision});
                        }
                        if (response) {
                            await reload(value);
                        }
                    });
                }
            });
        }
    });
    update();
    void run(() => reload());
};
