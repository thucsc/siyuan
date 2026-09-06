// 反链内嵌编辑器随内容撑开，定位需要滚动面板或所属正文。
export const getBacklinkScrollElement = (element: HTMLElement): HTMLElement | undefined => {
    const backlink = element.closest<HTMLElement>(".sy__backlink");
    if (!backlink) {
        return;
    }
    if (backlink.classList.contains("sy__backlink--bottom")) {
        return backlink.closest<HTMLElement>(".protyle-content") || undefined;
    }
    return element.closest<HTMLElement>(".backlinkList, .backlinkMList") || undefined;
};

// 单行或限高字段会裁剪引用，先移动字段内部视口，再定位外层面板。
export const revealBacklinkReference = (reference: HTMLElement, cell: HTMLElement) => {
    let parent = reference.parentElement;
    while (parent && cell.contains(parent)) {
        const rect = parent.getBoundingClientRect();
        const target = reference.getBoundingClientRect();
        if (parent.scrollWidth > parent.clientWidth && (target.left < rect.left || target.right > rect.right)) {
            parent.scrollLeft += target.left - rect.left;
        }
        if (parent.scrollHeight > parent.clientHeight && (target.top < rect.top || target.bottom > rect.bottom)) {
            parent.scrollTop += target.top - rect.top;
        }
        if (parent === cell) {
            break;
        }
        parent = parent.parentElement;
    }
};
