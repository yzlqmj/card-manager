const scanBtn = document.getElementById('scan-btn'); // 扫描按钮
const clearBtn = document.getElementById('clear-btn'); // 清除缓存按钮
const downloadBtn = document.getElementById('download-btn'); // 下载按钮
const showDownloaderBtn = document.getElementById('show-downloader-btn'); // 显示下载器按钮
const showLogBtn = document.getElementById('show-log-btn'); // 显示日志按钮
const showStatsBtn = document.getElementById('show-stats-btn'); // 显示统计信息按钮
const themeToggleBtn = document.getElementById('theme-toggle-btn'); // 主题切换按钮
const showFaceDownloaderBtn = document.getElementById('show-face-downloader-btn');
const container = document.getElementById('card-container'); // 卡片容器
const strayContainer = document.getElementById('stray-cards-container'); // 待整理卡片容器
const categorySelectFilter = document.getElementById('category-select-filter'); // Filter Select
const searchToggleBtn = document.getElementById('search-toggle-btn');
const searchPopover = document.getElementById('search-popover');
const searchInput = document.getElementById('search-input');
const filterToggleBtn = document.getElementById('filter-toggle-btn');
const filterPopover = document.getElementById('filter-popover');
const showUpdateNeededOnlyCheckbox = document.getElementById('show-update-needed-only');
const startListenClipboardBtn = document.getElementById('start-listen-clipboard-btn');
const stopListenClipboardBtn = document.getElementById('stop-listen-clipboard-btn');
const faceDownloadLog = document.getElementById('face-download-log');
const versionListElement = document.getElementById('details-version-list'); // 版本列表元素
const SERVER_URL = 'http://localhost:3600'; // 服务器地址
let allCardsData = {}; // 存储所有卡片数据
let currentCategories = []; // 当前分类列表
let fullDataset = {}; // 存储从服务器获取的完整数据
const markdownConverter = new showdown.Converter({ simpleLineBreaks: true }); // Markdown 转换器
let logHistory = []; // 日志历史记录

// --- 新的日志和通知系统 ---
function logMessage(message, type = 'info', details = '') {
    const timestamp = new Date().toLocaleTimeString(); // 获取当前时间戳
    const logEntry = `[${timestamp}] [${type.toUpperCase()}] ${message}${details ? `\n${details}` : ''}`; // 格式化日志条目
    logHistory.push(logEntry); // 将日志添加到历史记录
    showToast(message, type); // 显示通知
}

function showToast(message, type = 'info') {
    const toastContainer = document.getElementById('toast-container'); // 通知容器
    const toast = document.createElement('div'); // 创建通知元素
    toast.className = `toast ${type}`; // 设置通知样式
    toast.textContent = message; // 设置通知内容
    toastContainer.appendChild(toast); // 添加到容器中
    setTimeout(() => {
        toast.classList.add('show'); // 显示通知
    }, 10);
    setTimeout(() => {
        toast.classList.remove('show'); // 隐藏通知
        setTimeout(() => {
            toast.remove(); // 从 DOM 中移除通知
        }, 400);
    }, 5000); // 5 秒后自动隐藏
}

function showCustomConfirm(title, message, onConfirm) {
    const modal = document.getElementById('custom-confirm-modal');
    const titleEl = document.getElementById('confirm-title');
    const messageEl = document.getElementById('confirm-message');
    const okBtn = document.getElementById('confirm-ok-btn');
    const cancelBtn = document.getElementById('confirm-cancel-btn');

    titleEl.textContent = title;
    messageEl.textContent = message;

    // Removing old event listeners (cloning is a simple way to do this)
    const newOkBtn = okBtn.cloneNode(true);
    const newCancelBtn = cancelBtn.cloneNode(true);
    okBtn.parentNode.replaceChild(newOkBtn, okBtn);
    cancelBtn.parentNode.replaceChild(newCancelBtn, cancelBtn);

    newOkBtn.onclick = () => {
        onConfirm();
        closeModal('custom-confirm-modal');
    };

    newCancelBtn.onclick = () => {
        closeModal('custom-confirm-modal');
    };

    openModal('custom-confirm-modal');
}

function applyTheme(theme) {
    if (theme === 'dark') {
        document.body.classList.add('dark-theme'); // 应用深色主题
        themeToggleBtn.textContent = '☀️'; // 设置按钮图标
    } else {
        document.body.classList.remove('dark-theme'); // 应用浅色主题
        themeToggleBtn.textContent = '🌙'; // 设置按钮图标
    }
}
themeToggleBtn.addEventListener('click', () => {
    const newTheme = document.body.classList.contains('dark-theme') ? 'light' : 'dark'; // 切换主题
    localStorage.setItem('theme', newTheme); // 保存主题到本地存储
    applyTheme(newTheme); // 应用新主题
});

scanBtn.addEventListener('click', scanChanges); // 修改: 绑定到新的扫描函数
clearBtn.addEventListener('click', async () => {
    showCustomConfirm('清除缓存', '确定要清除所有本地缓存吗？这将导致下次扫描变慢。', async () => {
        try {
            const response = await fetch(`${SERVER_URL}/api/clear-cache`, { method: 'POST' }); // 发送清除缓存请求
            const result = await response.json();
            if (result.success) {
                container.innerHTML = ''; // 清空卡片容器
                strayContainer.innerHTML = ''; // 清空待整理容器
                logMessage('缓存已成功清除！', 'success'); // 记录成功日志
            } else {
                logMessage('清除缓存失败', 'error', result.message); // 记录失败日志
            }
        } catch (error) {
            logMessage('清除缓存请求失败', 'error', error.message); // 记录请求失败日志
        }
    });
});
showDownloaderBtn.addEventListener('click', () => {
    // 填充上次使用的值
    document.getElementById('character-name').value = localStorage.getItem('lastCharacterName') || '';
    document.getElementById('file-name').value = localStorage.getItem('lastFileName') || '';
    document.getElementById('category-select').value = localStorage.getItem('lastCategory') || '';
    // 清空URL输入框
    document.getElementById('download-url').value = '';
    openModal('downloader-modal');
});

showFaceDownloaderBtn.addEventListener('click', () => openFaceDownloader());

async function toggleClipboard(enable) {
    try {
        const response = await fetch(`${SERVER_URL}/api/toggle-clipboard?enable=${enable}`, { method: 'GET' });
        const result = await response.json();
        const action = enable ? '启用' : '关闭';
        if (response.ok) {
            showToast(`剪贴板监听已${action}`, 'success');
            logToFaceDownloader(`剪贴板监听已${action}。`);
        } else {
            showToast(`${action}监听失败`, 'error', result.message);
            logToFaceDownloader(`${action}监听失败: ${result.message}`);
        }
    } catch (error) {
        const action = enable ? '启用' : '关闭';
        showToast(`${action}监听请求失败`, 'error', error.message);
        logToFaceDownloader(`${action}监听请求失败: ${error.message}`);
    }
}

startListenClipboardBtn.addEventListener('click', () => toggleClipboard(true));
stopListenClipboardBtn.addEventListener('click', () => toggleClipboard(false));

showLogBtn.addEventListener('click', () => {
    const logContent = document.getElementById('log-content');
    logContent.textContent = logHistory.join('\n\n');
    openModal('log-modal');
});

showStatsBtn.addEventListener('click', async () => {
    try {
        const response = await fetch(`${SERVER_URL}/api/stats`);
        const result = await response.json();
        if (!response.ok) {
            throw new Error(result.message || '获取统计信息失败');
        }
        const stats = result.data;
        if (!stats) {
            throw new Error('无法获取统计信息');
        }

        const statsContent = document.getElementById('stats-content');
        statsContent.innerHTML = `
            <p><strong>总角色卡数量:</strong> ${stats.totalCharacters}</p>
            <p><strong>需要本地化的数量:</strong> ${stats.needsLocalization}</p>
            <p><strong>尚未本地化的数量:</strong> ${stats.notLocalized}</p>
            <p><strong>未导入的数量:</strong> ${stats.notImported}</p>
            <p><strong>未导入最新版的数量:</strong> ${stats.notLatestImported}</p>
        `;
        openModal('stats-modal');
    } catch (error) {
        logMessage('获取统计信息失败', 'error', error.message);
    }
});
downloadBtn.addEventListener('click', handleDownload);

versionListElement.addEventListener('click', (event) => {
    const target = event.target.closest('.version-list-item');
    if (!target) return;
    if (event.target.classList.contains('delete-btn')) {
        handleDeleteVersion(event.target.dataset.filepath);
    } else {
        updateDetailsPreview(target.dataset.imagepath);
        versionListElement.querySelectorAll('.version-list-item').forEach(el => el.classList.remove('active'));
        target.classList.add('active');
    }
});

document.addEventListener('DOMContentLoaded', () => {
    applyTheme(localStorage.getItem('theme') || 'light');

    const showUnimportedOnlyCheckbox = document.getElementById('show-unimported-only');
    const showNotLocalizedOnlyCheckbox = document.getElementById('show-not-localized-only');

    const savedUnimportedFilter = localStorage.getItem('showUnimportedOnly') === 'true';
    const savedNotLocalizedFilter = localStorage.getItem('showNotLocalizedOnly') === 'true';
    const savedUpdateNeededFilter = localStorage.getItem('showUpdateNeededOnly') === 'true';

    showUnimportedOnlyCheckbox.checked = savedUnimportedFilter;
    showNotLocalizedOnlyCheckbox.checked = savedNotLocalizedFilter;
    showUpdateNeededOnlyCheckbox.checked = savedUpdateNeededFilter;

    currentFilters.showUnimportedOnly = savedUnimportedFilter;
    currentFilters.showNotLocalizedOnly = savedNotLocalizedFilter;
    currentFilters.showUpdateNeededOnly = savedUpdateNeededFilter;

    fetchCards();

    showUnimportedOnlyCheckbox.addEventListener('change', (e) => {
        const isChecked = e.target.checked;
        localStorage.setItem('showUnimportedOnly', isChecked);
        applyFilters({ showUnimportedOnly: isChecked });
    });

    showNotLocalizedOnlyCheckbox.addEventListener('change', (e) => {
        const isChecked = e.target.checked;
        localStorage.setItem('showNotLocalizedOnly', isChecked);
        applyFilters({ showNotLocalizedOnly: isChecked });
    });

    showUpdateNeededOnlyCheckbox.addEventListener('change', (e) => {
        const isChecked = e.target.checked;
        localStorage.setItem('showUpdateNeededOnly', isChecked);
        applyFilters({ showUpdateNeededOnly: isChecked });
    });

    // Popover Management
    function togglePopover(popover, inputToFocus = null) {
        const isShown = popover.classList.contains('show');
        // Close all first
        document.querySelectorAll('.popover-menu').forEach(p => p.classList.remove('show'));
        if (!isShown) {
            popover.classList.add('show');
            if (inputToFocus) setTimeout(() => inputToFocus.focus(), 50);
        }
    }

    searchToggleBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        togglePopover(searchPopover, searchInput);
    });

    filterToggleBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        togglePopover(filterPopover);
    });

    // Prevent closing when clicking inside popover
    searchPopover.addEventListener('click', (e) => e.stopPropagation());
    filterPopover.addEventListener('click', (e) => e.stopPropagation());

    // Close on outside click
    document.addEventListener('click', () => {
        document.querySelectorAll('.popover-menu').forEach(p => p.classList.remove('show'));
    });

    // Search Input Logic
    searchInput.addEventListener('input', (e) => {
        applyFilters({ search: e.target.value.toLowerCase() });
    });

    // Category Select Logic
    categorySelectFilter.addEventListener('change', (e) => {
        const value = e.target.value;
        applyFilters({ category: value || null });
    });
});
function openModal(modalId) { document.getElementById(modalId).style.display = 'block'; }
function closeModal(modalId) { document.getElementById(modalId).style.display = 'none'; }
window.onclick = e => { if (e.target.classList.contains('modal')) closeModal(e.target.id); }

async function fetchCards() {
    container.innerHTML = '<div class="loader"></div>'; strayContainer.innerHTML = '';
    logMessage('正在加载卡片...');
    try {
        const response = await fetch(`${SERVER_URL}/api/cards`);
        const result = await response.json();
        if (!response.ok) throw new Error(result.message || '服务器错误');
        fullDataset = result.data; // 使用响应中的data字段
        renderAll(result.data, currentFilters);
        logMessage('卡片加载完成！', 'success');
    } catch (error) {
        container.innerHTML = '';
        logMessage('加载失败', 'error', error.message);
    }
}

async function scanChanges() {
    container.innerHTML = '<div class="loader"></div>'; strayContainer.innerHTML = '';
    logMessage('正在扫描变更...');
    try {
        const response = await fetch(`${SERVER_URL}/api/scan-changes`);
        const result = await response.json();
        if (!response.ok) throw new Error(result.message || '服务器错误');
        fullDataset = result.data; // 使用响应中的data字段
        renderAll(result.data, currentFilters);
        logMessage('扫描完成！', 'success');
    } catch (error) {
        container.innerHTML = '';
        logMessage('扫描失败', 'error', error.message);
    }
}

function renderAll(data, filters = {}) {
    allCardsData = {};
    if (data.categories) { Object.values(data.categories).flat().forEach(card => { allCardsData[card.folderPath] = card; }); }

    renderCategoryFilters(Object.keys(data.categories || {}), filters.category);
    renderStrayCards(data.strayCards || []);
    renderCategorizedCards(data.categories || {}, filters);
    updateCategoryDropdown(Object.keys(data.categories || {}));
    updateCharacterDatalist(Object.values(data.categories || {}).flat());
}

let currentFilters = {
    category: null,
    showUnimportedOnly: false,
    showNotLocalizedOnly: false,
    showUpdateNeededOnly: false,
    search: ''
};

function applyFilters(newFilter) {
    currentFilters = { ...currentFilters, ...newFilter };
    // 如果 newFilter 中没有提供 showUnimportedOnly，则保持 currentFilters 中已有的值不变
    if (newFilter && newFilter.showUnimportedOnly !== undefined) {
        currentFilters.showUnimportedOnly = newFilter.showUnimportedOnly;
    }
    if (newFilter && newFilter.showNotLocalizedOnly !== undefined) {
        currentFilters.showNotLocalizedOnly = newFilter.showNotLocalizedOnly;
    }
    if (newFilter && newFilter.showUpdateNeededOnly !== undefined) {
        currentFilters.showUpdateNeededOnly = newFilter.showUpdateNeededOnly;
    }
    if (newFilter && newFilter.search !== undefined) {
        currentFilters.search = newFilter.search;
    }
    renderAll(fullDataset, currentFilters);
}

function renderCategoryFilters(categoryNames, activeCategory) {
    categorySelectFilter.innerHTML = '<option value="">全部分类</option>';

    categoryNames.sort((a, b) => a.localeCompare(b, 'zh-Hans-CN')).forEach(name => {
        const option = document.createElement('option');
        option.value = name;
        option.textContent = name;
        if (name === activeCategory) option.selected = true;
        categorySelectFilter.appendChild(option);
    });
}

function renderStrayCards(strayCards) {
    if (!strayCards || strayCards.length === 0) { strayContainer.innerHTML = ''; return; }
    strayContainer.innerHTML = `<div class="category-section"><h2 class="category-title">待整理的卡片</h2><div class="card-grid"></div></div>`;
    const grid = strayContainer.querySelector('.card-grid');
    strayCards.forEach(card => {
        const cardElement = createCardElement(card.fileName, card.path, null, '', card.path, false);
        const organizeBtn = document.createElement('button');
        organizeBtn.textContent = '整理此卡';
        organizeBtn.style.cssText = 'margin: 16px; width: calc(100% - 32px); background-color: var(--warn-color); color: white; border: none; padding: 10px; border-radius: 6px; cursor: pointer;';
        organizeBtn.onclick = (e) => { e.stopPropagation(); handleOrganize(card.path); };
        cardElement.querySelector('.card-info').appendChild(organizeBtn);
        grid.appendChild(cardElement);
    });
}

function renderCategorizedCards(categories, filters = {}) {
    container.innerHTML = '';
    let categoryNames = Object.keys(categories);

    if (filters.category) {
        categoryNames = categoryNames.filter(name => name === filters.category);
    }

    const sortedCategoryNames = categoryNames.sort((a, b) => a.localeCompare(b, 'zh-Hans-CN'));

    for (const categoryName of sortedCategoryNames) {
        const cards = categories[categoryName];
        if (cards.length === 0) continue;
        const categorySection = document.createElement('div');
        categorySection.className = 'category-section';
        categorySection.innerHTML = `<h2 class="category-title">${categoryName}</h2><div class="card-grid"></div>`;
        const grid = categorySection.querySelector('.card-grid');
        let filteredCards = cards;
        if (filters.showUnimportedOnly) {
            filteredCards = filteredCards.filter(card => !card.importInfo.isImported);
        }
        if (filters.showNotLocalizedOnly) {
            filteredCards = filteredCards.filter(card => card.localizationNeeded && !card.isLocalized);
        }
        if (filters.showUpdateNeededOnly) {
            filteredCards = filteredCards.filter(card => card.importInfo && card.importInfo.isImported && !card.importInfo.isLatestImported);
        }

        if (filters.search) {
            const lowerSearch = filters.search.toLowerCase();
            filteredCards = filteredCards.filter(card =>
                card.name.toLowerCase().includes(lowerSearch) ||
                card.internalName.toLowerCase().includes(lowerSearch) ||
                card.folderPath.toLowerCase().includes(lowerSearch)
            );
        }

        if (filteredCards.length === 0) {
            continue;
        }

        filteredCards.sort((a, b) => a.name.localeCompare(b.name, 'zh-Hans-CN')).forEach(card => {
            const cardElement = createCardElement(card.internalName, card.latestVersionPath, card.importInfo, `版本数量: ${card.versionCount}`, card.folderPath, true);
            grid.appendChild(cardElement);
        });
        container.appendChild(categorySection);
    }
}

function createCardElement(name, path, importInfo, detailsText, key, isClickable) {
    const cardElement = document.createElement('div');
    cardElement.className = isClickable ? 'card is-clickable' : 'card';
    if (isClickable) { cardElement.dataset.key = key; cardElement.onclick = () => showDetails(key); }
    const imageUrl = `${SERVER_URL}/api/image?path=${encodeURIComponent(path)}`;
    let detailsHTML = `<p class="card-details">${detailsText}</p>`;
    if (importInfo) {
        const { isImported, isLatestImported, importedVersionPath } = importInfo;
        detailsHTML += isImported ? (isLatestImported ? '<span class="tag imported-ok">✓ 已导入最新版</span>' : `<span class="tag imported-warn" title="导入的版本: ${importedVersionPath}">⚠️ 已导入 (非最新)</span>`) : '<span class="tag not-imported">✗ 未导入</span>';
    }

    const cardData = allCardsData[key];
    if (cardData && cardData.localizationNeeded) {
        detailsHTML += cardData.isLocalized
            ? '<span class="tag localized-ok">✓ 已完成本地化</span>'
            : '<span class="tag not-localized">⚠️ 未完成本地化</span>';
    } else if (cardData) {
        detailsHTML += '<span class="tag localized-ok" style="background-color: transparent; border: 1px solid var(--border-color); color: var(--text-muted);">✓ 不需要本地化</span>';
    }

    cardElement.innerHTML = `<img src="${imageUrl}" alt="${name}" loading="lazy"><div class="card-info"><p class="card-name">${name}</p>${detailsHTML}</div>`;
    return cardElement;
}

function updateCategoryDropdown(categoryNames) {
    const sortedCategories = categoryNames.sort((a, b) => a.localeCompare(b, 'zh-Hans-CN'));
    currentCategories = sortedCategories;

    // 更新主下载器
    const downloadCategorySelect = document.getElementById('category-select');
    if (downloadCategorySelect) {
        downloadCategorySelect.innerHTML = '<option value="">选择一个现有分类</option>';
        sortedCategories.forEach(cat => {
            const option = document.createElement('option');
            option.value = cat;
            option.textContent = cat;
            downloadCategorySelect.appendChild(option);
        });
    }

    // 更新整理弹窗
    const organizeCategorySelect = document.getElementById('organize-category-select');
    organizeCategorySelect.innerHTML = '<option value="">选择一个现有分类</option>';
    sortedCategories.forEach(cat => {
        const option = document.createElement('option');
        option.value = cat;
        option.textContent = cat;
        organizeCategorySelect.appendChild(option);
    });
}

function updateFaceCharDatalist() {
    faceCharDatalist.innerHTML = '';
    Object.values(allCardsData).forEach(card => {
        const option = document.createElement('option');
        option.value = card.internalName;
        option.dataset.folderPath = card.folderPath;
        faceCharDatalist.appendChild(option);
    });
}

function updateDetailsPreview(imagePath) { document.getElementById('details-preview-img').src = `${SERVER_URL}/api/image?path=${encodeURIComponent(imagePath)}`; }

function showDetails(folderPath) {
    const card = allCardsData[folderPath];
    if (!card) {
        console.error("Card data not found for path:", folderPath);
        return;
    }

    // --- Basic Details ---
    document.getElementById('details-title').textContent = card.internalName;
    updateDetailsPreview(card.latestVersionPath);

    // --- Face Grid ---
    const faceGridContainer = document.getElementById('face-grid-container');
    const faceGrid = document.getElementById('face-grid');
    faceGridContainer.style.display = 'none';
    faceGrid.innerHTML = '';

    // --- Version List ---
    versionListElement.innerHTML = '';
    card.versions.forEach((v, index) => {
        const item = document.createElement('li');
        item.className = 'version-list-item';
        if (v.path === card.latestVersionPath) item.classList.add('active');
        item.dataset.imagepath = v.path;
        item.innerHTML = `<div class="version-item-info"><strong>${v.fileName}</strong><small>${v.path}</small></div><button class="delete-btn" data-filepath="${v.path}">删除</button>`;
        versionListElement.appendChild(item);
    });

    // --- Move Category Dropdown ---
    const moveSelect = document.getElementById('details-category-select');
    moveSelect.innerHTML = '';
    const currentCategoryMatch = folderPath.match(/.*[\\\/]([^\\\/]+)[\\\/][^\\\/]+$/);
    const currentCategory = currentCategoryMatch ? currentCategoryMatch[1] : '';
    currentCategories.forEach(cat => {
        if (cat === currentCategory) return;
        const option = document.createElement('option');
        option.value = cat;
        option.textContent = cat;
        moveSelect.appendChild(option);
    });

    // --- Button Clicks ---
    document.getElementById('details-move-btn').onclick = () => handleMove(card.folderPath);
    const actionsContainer = document.getElementById('details-actions');
    actionsContainer.innerHTML = ''; // 清空旧按钮

    const viewFacesBtn = document.createElement('button');
    viewFacesBtn.id = 'details-view-faces-btn';
    viewFacesBtn.textContent = '浏览卡面';
    if (card.hasFaceFolder) {
        viewFacesBtn.className = 'styled-btn primary';
        viewFacesBtn.onclick = () => showFaceViewer(card.folderPath);
    } else {
        viewFacesBtn.className = 'styled-btn primary';
        viewFacesBtn.disabled = true;
    }
    actionsContainer.appendChild(viewFacesBtn);

    const openFolderBtn = document.createElement('button');
    openFolderBtn.id = 'details-open-folder-btn';
    openFolderBtn.className = 'styled-btn primary';
    openFolderBtn.textContent = '打开角色文件夹';
    openFolderBtn.onclick = () => handleOpenFolder(card.folderPath);
    actionsContainer.appendChild(openFolderBtn);

    const localizeBtn = document.createElement('button');
    localizeBtn.id = 'details-localize-btn';

    if (card.localizationNeeded === false) {
        localizeBtn.textContent = '无需本地化';
    } else if (card.isLocalized) {
        localizeBtn.textContent = '重新本地化';
    } else { // needsLocalization is true or null, and not localized
        localizeBtn.textContent = '开始本地化';
    }
    // 统一设置为蓝色
    localizeBtn.className = 'styled-btn primary';

    // 始终允许点击
    localizeBtn.disabled = false;
    localizeBtn.onclick = () => handleLocalization(card.latestVersionPath);
    actionsContainer.appendChild(localizeBtn);

    const downloadFaceBtn = document.createElement('button');
    downloadFaceBtn.id = 'details-download-face-btn';
    downloadFaceBtn.className = 'styled-btn primary';
    downloadFaceBtn.textContent = '下载卡面';
    downloadFaceBtn.onclick = () => handleDownloadFace(card);
    actionsContainer.appendChild(downloadFaceBtn);

    const noteBtn = document.createElement('button');
    noteBtn.id = 'details-note-btn';
    noteBtn.className = 'styled-btn primary';
    noteBtn.textContent = '查看/编辑备注';
    noteBtn.onclick = () => showNoteModal(card.folderPath, card.internalName);
    actionsContainer.appendChild(noteBtn);

    const mergeBtn = document.createElement('button');
    mergeBtn.id = 'details-merge-btn';
    mergeBtn.className = 'styled-btn primary';
    mergeBtn.textContent = '合并JSON到PNG';
    mergeBtn.onclick = () => showMergeModal(card.folderPath);
    actionsContainer.appendChild(mergeBtn);

    // --- Show Modal ---
    openModal('details-modal');
}

function showNoteModal(folderPath, characterName) {
    const noteModalTitle = document.getElementById('note-modal-title');
    const noteDisplay = document.getElementById('note-display');
    const noteEdit = document.getElementById('note-edit');
    const editNoteBtn = document.getElementById('edit-note-btn');
    const saveNoteBtn = document.getElementById('save-note-btn');

    noteModalTitle.textContent = `备注 - ${characterName}`;

    const resetNoteState = () => {
        noteDisplay.style.display = 'block';
        noteEdit.style.display = 'none';
        editNoteBtn.style.display = 'inline-block';
        saveNoteBtn.style.display = 'none';
        noteDisplay.innerHTML = '<p class="note-placeholder"><i>正在加载备注...</i></p>';
        noteEdit.value = '';
    };

    const fetchNote = async () => {
        try {
            const response = await fetch(`${SERVER_URL}/api/note?folderPath=${encodeURIComponent(folderPath)}`);
            const result = await response.json();
            if (result.success && result.data) {
                const content = result.data.content || "";
                noteEdit.value = content;
                noteDisplay.innerHTML = content ? markdownConverter.makeHtml(content) : '<p><i>没有备注信息。点击“编辑”来添加。</i></p>';
            } else {
                // 如果获取失败（比如文件不存在），也允许用户编辑
                noteEdit.value = '';
                noteDisplay.innerHTML = `<p><i>没有备注信息或加载失败。点击“编辑”来创建。</i></p>`;
            }
        } catch (error) {
            noteEdit.value = '';
            noteDisplay.innerHTML = `<p style="color:red;">加载备注失败: ${error.message}。点击“编辑”来创建。</p>`;
        }
    };

    editNoteBtn.onclick = () => {
        noteDisplay.style.display = 'none';
        noteEdit.style.display = 'block';
        editNoteBtn.style.display = 'none';
        saveNoteBtn.style.display = 'inline-block';
        noteEdit.focus();
    };

    saveNoteBtn.onclick = async () => {
        const content = noteEdit.value;
        saveNoteBtn.disabled = true;
        saveNoteBtn.textContent = '保存中...';
        try {
            const response = await fetch(`${SERVER_URL}/api/note`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ folderPath, content })
            });
            const result = await response.json();
            if (result.success) {
                noteDisplay.innerHTML = content ? markdownConverter.makeHtml(content) : '<p><i>没有备注信息。点击“编辑”来添加。</i></p>';
                logMessage('备注已保存！', 'success');
                // 更新卡片数据中的 hasNote 状态
                if (allCardsData[folderPath]) {
                    allCardsData[folderPath].hasNote = !!content;
                }
            } else {
                logMessage('保存失败', 'error', result.message);
            }
        } catch (error) {
            logMessage('保存备注请求失败', 'error', error.message);
        } finally {
            noteDisplay.style.display = 'block';
            noteEdit.style.display = 'none';
            editNoteBtn.style.display = 'inline-block';
            saveNoteBtn.style.display = 'none';
            saveNoteBtn.disabled = false;
            saveNoteBtn.textContent = '保存';
        }
    };

    resetNoteState();
    fetchNote();
    openModal('note-modal');
}

async function handleOpenFolder(folderPath) {
    try {
        const response = await fetch(`${SERVER_URL}/api/open-folder`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ folderPath })
        });

        const result = await response.json();

        if (result.success) {
            logMessage(result.message || '文件夹已打开', 'success');
        } else {
            logMessage(result.error || '打开文件夹失败', 'error');
        }
    } catch (error) {
        logMessage('打开文件夹请求失败', 'error', error.message);
        console.error('Open folder request failed:', error);
    }
}

async function handleDeleteVersion(filePath) {
    const fileName = filePath.substring(filePath.lastIndexOf(/[\\\/]/) + 1);
    showCustomConfirm('删除文件', `确定要删除文件: ${fileName} 吗？\n此操作不可恢复！`, async () => {
        try {
            const response = await fetch(`${SERVER_URL}/api/delete-version`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ filePath }) });
            const result = await response.json();

            if (result.success) {
                logMessage(result.message || '删除成功', 'success');
                closeModal('details-modal');
                fetchCards();
            } else {
                logMessage(result.error || '删除失败', 'error');
            }
        } catch (error) { logMessage('删除版本请求失败', 'error', error.message); }
    });
}

async function handleMove(oldFolderPath) {
    const newCategory = document.getElementById('details-category-select').value;
    if (!newCategory) { showToast('请选择一个目标分类！', 'error'); return; }
    const characterName = oldFolderPath.substring(oldFolderPath.lastIndexOf(/[\\\/]/) + 1);
    showCustomConfirm('移动分类', `确定要将角色 '${characterName}' 移动到分类 '${newCategory}' 吗？`, async () => {
        try {
            const response = await fetch(`${SERVER_URL}/api/move-character`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ oldFolderPath, newCategory }) });
            const result = await response.json();

            if (result.success) {
                logMessage(result.message || '移动成功', 'success');
                closeModal('details-modal');
                fetchCards();
            } else {
                logMessage(result.error || '移动失败', 'error');
            }
        } catch (error) { logMessage('移动角色请求失败', 'error', error.message); }
    });
}

async function handleDownload() {
    const url = document.getElementById('download-url').value.trim();
    const characterName = document.getElementById('character-name').value.trim();
    const fileName = document.getElementById('file-name').value.trim() || characterName;
    let category = document.getElementById('category-select').value;
    const newCategory = document.getElementById('new-category').value.trim();
    if (newCategory) category = newCategory;
    if (!url || !characterName || !category) {
        logMessage('链接、角色名和分类为必填项！', 'error');
        return;
    }
    logMessage('正在下载中...');
    try {
        const response = await fetch(`${SERVER_URL}/api/download-card`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ url, category, characterName, fileName }) });
        if (!response.ok) {
            const result = await response.json();
            logMessage(result.message || '下载失败', 'error');
        } else {
            const result = await response.json();
            logMessage(result.message || '下载成功', 'success');
            localStorage.setItem('lastCharacterName', characterName);
            localStorage.setItem('lastFileName', fileName);
            localStorage.setItem('lastCategory', category);
            closeModal('downloader-modal');
            fetchCards();
        }
    } catch (error) { logMessage('下载请求失败', 'error', error.message); }
}

function handleOrganize(strayPath) {
    const modal = document.getElementById('organize-modal');
    const previewImg = document.getElementById('organize-preview-img');
    const charNameInput = document.getElementById('organize-char-name');
    const categorySelect = document.getElementById('organize-category-select');
    const newCategoryInput = document.getElementById('organize-new-category');
    const moveBtn = document.getElementById('organize-move-btn');
    const deleteBtn = document.getElementById('organize-delete-btn');

    const defaultName = strayPath.substring(strayPath.lastIndexOf(/[\\\/]/) + 1).replace(/\.png$/i, '');
    previewImg.src = `${SERVER_URL}/api/image?path=${encodeURIComponent(strayPath)}`;
    charNameInput.value = defaultName;
    newCategoryInput.value = '';
    categorySelect.selectedIndex = 0;

    moveBtn.onclick = async () => {
        const characterName = charNameInput.value.trim();
        let category = categorySelect.value;
        const newCategory = newCategoryInput.value.trim();
        if (newCategory) {
            category = newCategory;
        }
        if (!characterName || !category) {
            logMessage('角色名和分类为必填项！', 'error');
            return;
        }
        logMessage('正在整理文件...');
        try {
            const response = await fetch(`${SERVER_URL}/api/organize-stray`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ strayPath, category, characterName }) });
            const result = await response.json();

            if (result.success) {
                logMessage(result.message || '整理成功', 'success');
                closeModal('organize-modal');
                fetchCards();
            } else {
                logMessage(result.error || '整理失败', 'error');
            }
        } catch (error) {
            logMessage('整理请求失败', 'error', error.message);
        }
    };

    deleteBtn.onclick = () => {
        const fileName = strayPath.substring(strayPath.lastIndexOf(/[\\\/]/) + 1);
        showCustomConfirm('删除文件', `确定要永久删除待整理文件: ${fileName} 吗？\n此操作不可恢复！`, async () => {
            try {
                const response = await fetch(`${SERVER_URL}/api/delete-stray`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ filePath: strayPath }) });
                const result = await response.json();

                if (result.success) {
                    logMessage(result.message || '删除成功', 'success');
                    closeModal('organize-modal');
                    fetchCards();
                } else {
                    logMessage(result.error || '删除失败', 'error');
                }
            } catch (error) {
                logMessage('删除请求失败', 'error', error.message);
            }
        });
    };

    openModal('organize-modal');
}

function updateCharacterDatalist(cards) {
    const datalist = document.getElementById('character-list');
    datalist.innerHTML = '';
    const characterNames = new Set();
    cards.forEach(card => {
        if (!characterNames.has(card.name)) {
            characterNames.add(card.name);
            const option = document.createElement('option');
            option.value = card.name;
            datalist.appendChild(option);
        }
    });

    // 当角色名称变化时，自动选择分类
    const characterNameInput = document.getElementById('character-name');
    characterNameInput.addEventListener('change', () => {
        const selectedName = characterNameInput.value;
        const card = cards.find(c => c.name === selectedName);
        if (card) {
            const folderPath = card.folderPath;
            const categoryMatch = folderPath.match(/.*[\\\/]([^\\\/]+)[\\\/][^\\\/]+$/);
            if (categoryMatch && categoryMatch[1]) {
                const category = categoryMatch[1];
                document.getElementById('category-select').value = category;
                document.getElementById('new-category').value = ''; // 清空新分类输入
            }
        }
    });
}

let submittedUrlPoller = null;

let faceDownloadTarget = {
    folderPath: null
};

function startUrlPolling() {
    if (submittedUrlPoller) return; // Prevent multiple pollers
    logToFaceDownloader('开始从队列获取URL...');
    submittedUrlPoller = setInterval(async () => {
        // 优先使用从输入框选择的角色
        const selectedCharName = faceCharInput.value;
        const options = Array.from(faceCharDatalist.options);
        const selectedOption = options.find(opt => opt.value === selectedCharName);
        const selectedCharFolder = selectedOption ? selectedOption.dataset.folderPath : null;

        // 如果输入框有有效选择，则更新目标
        if (selectedCharFolder) {
            faceDownloadTarget.folderPath = selectedCharFolder;
        }

        // 如果没有目标文件夹，则不执行任何操作
        if (!faceDownloadTarget.folderPath) {
            return;
        }

        try {
            const response = await fetch(`${SERVER_URL}/api/get-submitted-url`);
            if (!response.ok) return;

            const result = await response.json();
            if (result.success && result.data.url) {
                logToFaceDownloader(`从队列中获取链接: ${result.data.url}`);
                await downloadFaceImage(result.data.url, faceDownloadTarget.folderPath);
            }
        } catch (error) {
            // 忽略网络错误
        }
    }, 2500);
}


function stopUrlPolling() {
    if (submittedUrlPoller) {
        clearInterval(submittedUrlPoller);
        submittedUrlPoller = null;
        logToFaceDownloader('已停止从队列获取URL。');
    }
}

// 当打开或关闭卡面下载模态框时，启动或停止轮询
// 移除旧的 observer 逻辑


async function downloadFaceImage(url, characterFolderPath) {
    logToFaceDownloader(`正在下载卡面: ${url}`);

    // 从 characterFolderPath 中提取 category 和 characterName
    // 格式通常是 "Tavern/characters/分类/角色名"
    const pathParts = characterFolderPath.replace(/\\/g, '/').split('/');
    if (pathParts.length < 2) {
        logToFaceDownloader(`下载失败: 角色路径格式不正确 "${characterFolderPath}"`);
        showToast('下载失败: 角色路径格式不正确', 'error');
        return;
    }
    const characterName = pathParts.pop();
    const category = pathParts.pop();

    try {
        // 复用角色卡下载的API
        const response = await fetch(`${SERVER_URL}/api/download-card`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                url: url,
                category: category,
                characterName: characterName,
                fileName: '', // 文件名留空，让后端自动生成
                isFace: true // 添加一个标志，告诉后端这是卡面下载
            })
        });

        const result = await response.json();
        if (response.ok) {
            logToFaceDownloader(`下载成功: ${result.message}`);
            showToast('卡面下载成功!', 'success');
        } else {
            logToFaceDownloader(`下载失败: ${result.message}`);
            showToast(`下载失败: ${result.message}`, 'error');
        }
    } catch (error) {
        logToFaceDownloader(`下载请求失败: ${error.message}`);
        showToast('下载请求失败', 'error');
    }
}

async function handleLocalization(cardPath) {
    logMessage('开始本地化...');
    openModal('localization-log-modal');
    const logContent = document.getElementById('localization-log-content');
    logContent.textContent = '正在调用本地化程序...\n';

    try {
        const response = await fetch(`${SERVER_URL}/api/localize-card`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ cardPath })
        });

        if (!response.ok) {
            logContent.textContent = `本地化失败: HTTP ${response.status}`;
            logMessage('本地化失败', 'error', `HTTP ${response.status}`);
            return;
        }

        // 检查是否是流式响应
        const contentType = response.headers.get('content-type');
        if (contentType && contentType.includes('text/event-stream')) {
            // 处理流式响应
            const reader = response.body.getReader();
            const decoder = new TextDecoder();
            let buffer = '';
            let linkCount = 0;
            let successCount = 0;
            let failureCount = 0;

            while (true) {
                const { done, value } = await reader.read();
                if (done) break;

                buffer += decoder.decode(value, { stream: true });
                const lines = buffer.split('\n');
                buffer = lines.pop(); // 保留不完整的行

                for (const line of lines) {
                    if (line.startsWith('data: ')) {
                        try {
                            const data = JSON.parse(line.slice(6));
                            const timestamp = new Date().toLocaleTimeString();

                            switch (data.type) {
                                case 'links':
                                    logContent.textContent += `\n${data.content}\n`;
                                    break;
                                case 'link':
                                    linkCount++;
                                    logContent.textContent += `${data.content}\n`;
                                    break;
                                case 'separator':
                                    logContent.textContent += `${'─'.repeat(40)}\n`;
                                    break;
                                case 'success':
                                    successCount++;
                                    logContent.textContent += `✅ ${data.content}\n`;
                                    break;
                                case 'failure':
                                    failureCount++;
                                    logContent.textContent += `❌ ${data.content}\n`;
                                    break;
                                case 'stats-ok':
                                    logContent.textContent += `\n${data.content}\n`;
                                    break;
                                case 'stats-warn':
                                    logContent.textContent += `\n${data.content}\n`;
                                    break;
                                case 'failed-title':
                                    logContent.textContent += `${data.content}\n`;
                                    break;
                                case 'failed-link':
                                    logContent.textContent += `${data.content}\n`;
                                    break;
                                case 'complete':
                                    logContent.textContent += `\n🎉 完成！\n`;
                                    logMessage('本地化成功！', 'success');
                                    fetchCards();
                                    return;
                            }

                            // 自动滚动到底部
                            logContent.scrollTop = logContent.scrollHeight;
                        } catch (e) {
                            console.warn('解析SSE数据失败:', e, line);
                        }
                    }
                }
            }
        } else {
            // 处理传统JSON响应（向后兼容）
            const result = await response.json();
            logContent.textContent = result.log;
            logMessage('本地化成功！', 'success');
            fetchCards(); // 重新加载卡片以更新状态
        }
    } catch (error) {
        logContent.textContent = `本地化请求失败: ${error.message}`;
        logMessage('本地化请求失败', 'error', error.message);
    }
}

function logToFaceDownloader(message) {
    const timestamp = new Date().toLocaleTimeString();
    faceDownloadLog.textContent = `[${timestamp}] ${message}\n` + faceDownloadLog.textContent;
}

async function showFaceViewer(folderPath) {
    const faceGridContainer = document.getElementById('face-grid-container');
    const faceGrid = document.getElementById('face-grid');

    // 如果已经显示，则隐藏
    if (faceGridContainer.style.display === 'block') {
        faceGridContainer.style.display = 'none';
        return;
    }

    faceGrid.innerHTML = '<div class="loader"></div>';
    faceGridContainer.style.display = 'block';

    try {
        const response = await fetch(`${SERVER_URL}/api/faces?characterFolderPath=${encodeURIComponent(folderPath)}`);
        const result = await response.json();

        faceGrid.innerHTML = ''; // 清空加载动画

        if (result.success && result.data.faces.length > 0) {
            result.data.faces.forEach(imagePath => {
                const img = document.createElement('img');
                img.src = `${SERVER_URL}/api/image?path=${encodeURIComponent(imagePath)}`;
                img.alt = 'Card Face';
                img.loading = 'lazy';
                img.onclick = () => window.open(img.src, '_blank');
                faceGrid.appendChild(img);
            });
        } else if (result.success) {
            faceGrid.innerHTML = '<p>该角色没有卡面图片。</p>';
        } else {
            faceGrid.innerHTML = `<p style="color:red;">无法加载卡面图片: ${result.message}</p>`;
        }
    } catch (error) {
        faceGrid.innerHTML = `<p style="color:red;">请求卡面图片失败: ${error.message}</p>`;
    }
}

function openFaceDownloader(card = null) {
    // 重置状态
    faceDownloadTarget.folderPath = null;
    faceDownloadLog.textContent = '';
    updateFaceCharDatalist();

    if (card) {
        // 从角色详情页打开
        faceDownloadTarget.folderPath = card.folderPath;
        faceCharInput.value = card.internalName;
        faceCharInput.disabled = true;
        logToFaceDownloader(`已锁定角色: ${card.internalName}`);
    } else {
        // 从顶部按钮打开
        faceCharInput.value = '';
        faceCharInput.disabled = false;
        logToFaceDownloader('请在上方选择一个角色以开始下载。');
    }

    openModal('face-downloader-modal');
    toggleClipboard(true);
    startUrlPolling();
}

function handleDownloadFace(card) {
    openFaceDownloader(card);
}

// 将清理逻辑移到 closeModal 中
const originalCloseModal = closeModal;
closeModal = function (modalId) {
    if (modalId === 'face-downloader-modal') {
        stopUrlPolling();
        toggleClipboard(false);
        faceDownloadTarget.folderPath = null;
    }
    originalCloseModal(modalId);
};

async function showMergeModal(folderPath) {
    const jsonSelect = document.getElementById('merge-json-select');
    const pngSelect = document.getElementById('merge-png-select');
    const confirmBtn = document.getElementById('merge-confirm-btn');

    jsonSelect.innerHTML = '<option value="">加载中...</option>';
    pngSelect.innerHTML = '<option value="">加载中...</option>';
    confirmBtn.disabled = true;

    try {
        const response = await fetch(`${SERVER_URL}/api/list-files?folderPath=${encodeURIComponent(folderPath)}`);
        if (!response.ok) {
            throw new Error('无法获取文件列表');
        }
        const data = await response.json();

        jsonSelect.innerHTML = '<option value="">--选择一个JSON文件--</option>';
        data.jsonFiles.forEach(file => {
            const option = document.createElement('option');
            option.value = file;
            option.textContent = file;
            jsonSelect.appendChild(option);
        });

        pngSelect.innerHTML = '<option value="">--选择一个PNG文件--</option>';
        data.pngFiles.forEach(file => {
            const option = document.createElement('option');
            option.value = file;
            option.textContent = file;
            pngSelect.appendChild(option);
        });

        confirmBtn.disabled = false;
        confirmBtn.onclick = async () => {
            const jsonFile = jsonSelect.value;
            const pngFile = pngSelect.value;

            if (!jsonFile || !pngFile) {
                logMessage('请同时选择一个 JSON 和一个 PNG 文件！', 'error');
                return;
            }

            confirmBtn.disabled = true;
            confirmBtn.textContent = '合并中...';

            try {
                const mergeResponse = await fetch(`${SERVER_URL}/api/merge-json-to-png`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        folderPath: folderPath,
                        jsonFileName: jsonFile,
                        pngFileName: pngFile
                    })
                });
                const result = await mergeResponse.json();
                if (!mergeResponse.ok) {
                    throw new Error(result.message || '合并失败');
                }
                logMessage(result.message, 'success');
                closeModal('merge-modal');
                fetchCards(); // 刷新列表以显示新文件
            } catch (error) {
                logMessage(`合并失败: ${error.message}`, 'error');
            } finally {
                confirmBtn.disabled = false;
                confirmBtn.textContent = '确认合并';
            }
        };

        openModal('merge-modal');

    } catch (error) {
        logMessage(`无法加载文件列表: ${error.message}`, 'error');
        jsonSelect.innerHTML = '<option value="">加载失败</option>';
        pngSelect.innerHTML = '<option value="">加载失败</option>';
    }
}