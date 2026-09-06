import {setAVLocateRequest} from "./locate";
import {getBacklinkScrollElement} from "./backlinkScroll";

export interface IBacklinkAVTarget {
    blockID: string;
    matches: {
        itemID: string;
        keyID: string;
        valueID: string;
        title: string;
        keyName: string;
        defIDs: string[];
    }[];
}

// 每个反链副本独立定位，避免同 ID 的正文或其他反链数据库接收到定位请求。
export const prepareBacklinkAV = (element: HTMLElement, targets: IBacklinkAVTarget[], scrollState?: {pending: boolean}) => {
    const databases = element.matches(".av[data-node-id]") ? [element] :
        Array.from(element.querySelectorAll<HTMLElement>(".av[data-node-id]"));
    databases.forEach(database => {
        const target = targets.find(item => item.blockID === database.dataset.nodeId);
        if (!target?.matches.length) {
            return;
        }
        const match = target.matches[0];
        database.querySelectorAll(".def--mark").forEach(item => item.classList.remove("def--mark"));
        // 首次展开仅滚动到首个数据库命中，刷新和其他副本保留阅读位置。
        setAVLocateRequest(database, {
            itemID: match.itemID,
            keyID: match.keyID,
            defIDs: match.defIDs,
            select: false,
            highlight: true,
            persistView: false,
            scroll: scrollState?.pending === true,
            scrollElement: getBacklinkScrollElement(database),
        });
        if (scrollState) {
            scrollState.pending = false;
        }
    });
};
