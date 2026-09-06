import {setAVLocateRequest} from "./locate";
import {avRender} from "./render";

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
export const prepareBacklinkAV = (element: HTMLElement, protyle: IProtyle, targets: IBacklinkAVTarget[],
                                 selections: Map<string, string>, scrollState?: {pending: boolean}) => {
    const databases = element.matches(".av[data-node-id]") ? [element] :
        Array.from(element.querySelectorAll<HTMLElement>(".av[data-node-id]"));
    databases.forEach(database => {
        const target = targets.find(item => item.blockID === database.dataset.nodeId);
        if (!target?.matches.length) {
            return;
        }
        const navigation = document.createElement("div");
        navigation.className = "fn__flex fn__flex-center";
        navigation.classList.toggle("fn__none", database.classList.contains("fn__none"));
        navigation.contentEditable = "false";
        const select = document.createElement("select");
        select.className = "b3-select fn__flex-1";
        select.style.minWidth = "0";
        select.setAttribute("aria-label", window.siyuan.languages.ref);
        target.matches.forEach((match, index) => {
            const option = document.createElement("option");
            option.value = index.toString();
            option.textContent = `${index + 1}/${target.matches.length} ${match.title || window.siyuan.languages.untitled} - ${match.keyName}`;
            select.appendChild(option);
        });
        const previousIndex = target.matches.findIndex(match => match.valueID === selections.get(target.blockID));
        select.selectedIndex = Math.max(0, previousIndex);
        const locate = (scroll: boolean) => {
            const match = target.matches[select.selectedIndex];
            selections.set(target.blockID, match.valueID);
            database.querySelectorAll(".def--mark").forEach(item => item.classList.remove("def--mark"));
            setAVLocateRequest(database, {
                itemID: match.itemID,
                keyID: match.keyID,
                defIDs: match.defIDs,
                select: false,
                highlight: true,
                persistView: false,
                scroll,
            });
        };
        const navigate = () => {
            if (!database.isConnected) {
                return;
            }
            locate(true);
            database.removeAttribute("data-render");
            void avRender(database, protyle);
        };
        select.addEventListener("change", navigate);
        navigation.appendChild(select);
        const button = document.createElement("button");
        button.type = "button";
        button.className = "b3-button b3-button--outline fn__flex-shrink";
        button.textContent = window.siyuan.languages.jumpTo;
        button.addEventListener("click", navigate);
        navigation.appendChild(button);
        // 控件位于数据库块外，数据库重新渲染不会移除命中位置列表。
        ["click", "mousedown", "keydown", "change"].forEach(type => {
            navigation.addEventListener(type, event => event.stopPropagation());
        });
        database.before(navigation);
        // 首次展开仅滚动到首个数据库命中，刷新和其他副本保留阅读位置。
        locate(scrollState?.pending === true);
        if (scrollState) {
            scrollState.pending = false;
        }
    });
};
