// Main JavaScript for wikigo
// Note: Theme initialization is in base.html <head> to prevent blink

function toggleTheme() {
    const isDark = document.documentElement.getAttribute('data-theme') === 'dark';
    if (isDark) {
        document.documentElement.removeAttribute('data-theme');
        localStorage.setItem('theme', 'light');
    } else {
        document.documentElement.setAttribute('data-theme', 'dark');
        localStorage.setItem('theme', 'dark');
    }
}

function runInPlayground(btn) {
    const exampleBody = btn.closest('.Example-body');
    const codeBlock = exampleBody.querySelector('.Example-code code');
    if (!codeBlock) return;

    let code = codeBlock.textContent;

    // Wrap in main package if needed
    if (!code.includes('package ')) {
        code = 'package main\n\nimport "fmt"\n\nfunc main() {\n' + code + '\n}';
    }

    // Check if embed already exists
    let embedContainer = exampleBody.querySelector('.PlaygroundEmbed');
    if (embedContainer) {
        // Toggle visibility
        embedContainer.style.display = embedContainer.style.display === 'none' ? 'block' : 'none';
        btn.textContent = embedContainer.style.display === 'none' ? 'Run' : 'Hide';
        return;
    }

    btn.textContent = 'Loading...';
    btn.disabled = true;

    // Use fetch to get share ID, then create embed
    fetch('https://go.dev/_/share', {
        method: 'POST',
        body: code,
    })
    .then(response => response.text())
    .then(shareId => {
        // Create embed container
        embedContainer = document.createElement('div');
        embedContainer.className = 'PlaygroundEmbed';

        // Create iframe for the playground
        const iframe = document.createElement('iframe');
        iframe.src = `https://go.dev/play/p/${shareId}?v=gotip`;
        iframe.className = 'PlaygroundEmbed-iframe';
        iframe.setAttribute('sandbox', 'allow-scripts allow-same-origin allow-popups');

        // Add close button
        const closeBtn = document.createElement('button');
        closeBtn.className = 'PlaygroundEmbed-close';
        closeBtn.textContent = 'Close';
        closeBtn.onclick = () => {
            embedContainer.style.display = 'none';
            btn.textContent = 'Run';
        };

        // Add open in new tab link
        const openLink = document.createElement('a');
        openLink.href = `https://go.dev/play/p/${shareId}`;
        openLink.target = '_blank';
        openLink.className = 'PlaygroundEmbed-open';
        openLink.textContent = 'Open in new tab';

        const controls = document.createElement('div');
        controls.className = 'PlaygroundEmbed-controls';
        controls.appendChild(openLink);
        controls.appendChild(closeBtn);

        embedContainer.appendChild(controls);
        embedContainer.appendChild(iframe);
        exampleBody.appendChild(embedContainer);

        btn.textContent = 'Hide';
        btn.disabled = false;
    })
    .catch(() => {
        // Fallback: open playground in new tab
        window.open('https://go.dev/play/', '_blank');
        btn.textContent = 'Run';
        btn.disabled = false;
    });
}

function formatExample(btn) {
    const exampleBody = btn.closest('.Example-body');
    const codeBlock = exampleBody.querySelector('.Example-code code');
    if (!codeBlock) return;

    const originalText = btn.textContent;
    btn.textContent = 'Formatting...';
    btn.disabled = true;

    let code = codeBlock.textContent;

    // Wrap in package if needed for formatting
    let wrapped = false;
    if (!code.includes('package ')) {
        code = 'package main\n\nfunc example() {\n' + code + '\n}';
        wrapped = true;
    }

    fetch('https://go.dev/_/fmt', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: 'body=' + encodeURIComponent(code),
    })
    .then(response => response.json())
    .then(data => {
        if (data.Body) {
            let formatted = data.Body;
            // Unwrap if we wrapped it
            if (wrapped) {
                const match = formatted.match(/func example\(\) \{\n([\s\S]*)\n\}$/);
                if (match) {
                    formatted = match[1].split('\n').map(line =>
                        line.startsWith('\t') ? line.slice(1) : line
                    ).join('\n');
                }
            }
            codeBlock.textContent = formatted;
            if (typeof Prism !== 'undefined') {
                Prism.highlightElement(codeBlock);
            }
        } else if (data.Error) {
            console.error('Format error:', data.Error);
        }
    })
    .catch(err => console.error('Format failed:', err))
    .finally(() => {
        btn.textContent = originalText;
        btn.disabled = false;
    });
}

function shareExample(btn) {
    const example = btn.closest('.Example');
    const id = example ? example.id : null;

    let url = window.location.href.split('#')[0];
    if (id) {
        url += '#' + id;
    }

    navigator.clipboard.writeText(url).then(() => {
        const originalText = btn.textContent;
        btn.textContent = 'Copied!';
        setTimeout(() => {
            btn.textContent = originalText;
        }, 1500);
    });
}

function showImports() {
    const popup = document.getElementById('importsPopup');
    if (popup) popup.classList.add('open');
}

function hideImports() {
    const popup = document.getElementById('importsPopup');
    if (popup) popup.classList.remove('open');
}

function toggleType(btn) {
    const li = btn.closest('.Index-type');
    const sublist = li.nextElementSibling;
    if (sublist && sublist.classList.contains('Index-sublist')) {
        const collapsed = sublist.getAttribute('data-collapsed') === 'true';
        sublist.setAttribute('data-collapsed', collapsed ? 'false' : 'true');
        btn.textContent = collapsed ? '-' : '+';
    }
}

function toggleAllTypes(btn) {
    const sublists = document.querySelectorAll('.Index-sublist');
    const buttons = document.querySelectorAll('.Index-typeToggle');
    const expanding = btn.textContent === 'Expand All';

    sublists.forEach(sublist => {
        sublist.setAttribute('data-collapsed', expanding ? 'false' : 'true');
    });
    buttons.forEach(b => {
        b.textContent = expanding ? '-' : '+';
    });
    btn.textContent = expanding ? 'Collapse All' : 'Expand All';
}

function copyImportPath(btn) {
    const path = btn.dataset.path;
    navigator.clipboard.writeText(path).then(() => {
        btn.textContent = 'Copied!';
        setTimeout(() => { btn.textContent = 'Copy'; }, 1500);
    });
}

document.addEventListener('DOMContentLoaded', function() {
    // Theme toggle button
    const themeToggle = document.getElementById('themeToggle');
    if (themeToggle) {
        themeToggle.addEventListener('click', toggleTheme);
    }

    // Mobile menu toggle
    const menuBtn = document.getElementById('menuBtn');
    const headerNav = document.getElementById('headerNav');
    if (menuBtn && headerNav) {
        menuBtn.addEventListener('click', () => {
            headerNav.classList.toggle('open');
        });
    }

    // Imports popup - click outside to close
    const importsPopup = document.getElementById('importsPopup');
    if (importsPopup) {
        importsPopup.addEventListener('click', (e) => {
            if (e.target === importsPopup) {
                hideImports();
            }
        });
        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape' && importsPopup.classList.contains('open')) {
                hideImports();
            }
        });
    }

    // Initialize Prism syntax highlighting
    if (typeof Prism !== 'undefined') {
        Prism.highlightAll();
    }

    // Smooth scroll for anchor links
    document.querySelectorAll('a[href^="#"]').forEach(anchor => {
        anchor.addEventListener('click', function(e) {
            const targetId = this.getAttribute('href').slice(1);
            const target = document.getElementById(targetId);
            if (target) {
                e.preventDefault();
                const headerHeight = document.querySelector('.Header').offsetHeight;
                const targetPosition = target.getBoundingClientRect().top + window.pageYOffset - headerHeight - 20;
                window.scrollTo({
                    top: targetPosition,
                    behavior: 'smooth'
                });
                // Update URL hash
                history.pushState(null, null, '#' + targetId);
            }
        });
    });

    // Highlight current section in navigation
    const navLinks = document.querySelectorAll('.Package-navList a');
    const sections = [];

    navLinks.forEach(link => {
        const href = link.getAttribute('href');
        if (href && href.startsWith('#')) {
            const section = document.getElementById(href.slice(1));
            if (section) {
                sections.push({ link, section });
            }
        }
    });

    function updateActiveNav() {
        const headerHeight = document.querySelector('.Header')?.offsetHeight || 0;
        const scrollPos = window.scrollY + headerHeight + 50;

        let currentSection = null;
        sections.forEach(({ link, section }) => {
            if (section.offsetTop <= scrollPos) {
                currentSection = link;
            }
        });

        navLinks.forEach(link => link.classList.remove('active'));
        if (currentSection) {
            currentSection.classList.add('active');
        }
    }

    if (sections.length > 0) {
        window.addEventListener('scroll', updateActiveNav, { passive: true });
        updateActiveNav();
    }

    // Expand example on hash navigation
    if (window.location.hash) {
        const target = document.querySelector(window.location.hash);
        if (target && target.tagName === 'DETAILS') {
            target.open = true;
        }
    }

    // Copy code button
    document.querySelectorAll('pre code').forEach(block => {
        const pre = block.parentElement;
        const button = document.createElement('button');
        button.className = 'copy-button';
        button.textContent = 'Copy';
        button.addEventListener('click', async () => {
            try {
                await navigator.clipboard.writeText(block.textContent);
                button.textContent = 'Copied!';
                setTimeout(() => {
                    button.textContent = 'Copy';
                }, 2000);
            } catch (err) {
                console.error('Failed to copy:', err);
            }
        });
        pre.style.position = 'relative';
        pre.appendChild(button);
    });

    // Lazy loading for large packages
    initLazyLoading();

    // Search form enhancement with autocomplete
    const searchInput = document.querySelector('.SearchForm-input');
    if (searchInput) {
        initSearchAutocomplete(searchInput);
        document.addEventListener('keydown', (e) => {
            // / to focus search
            if (e.key === '/' && !isInputFocused()) {
                e.preventDefault();
                searchInput.focus();
            }
            // Escape to blur search
            if (e.key === 'Escape' && document.activeElement === searchInput) {
                searchInput.blur();
                hideAutocomplete();
            }
        });
    }
});

// Search autocomplete
let autocompleteTimeout = null;
let autocompleteContainer = null;

function initSearchAutocomplete(input) {
    // Create autocomplete container
    autocompleteContainer = document.createElement('div');
    autocompleteContainer.className = 'SearchAutocomplete';
    input.parentElement.style.position = 'relative';
    input.parentElement.appendChild(autocompleteContainer);

    let selectedIndex = -1;

    input.addEventListener('input', (e) => {
        const query = e.target.value.trim();
        if (query.length < 2) {
            hideAutocomplete();
            return;
        }

        clearTimeout(autocompleteTimeout);
        autocompleteTimeout = setTimeout(() => fetchSuggestions(query), 200);
    });

    input.addEventListener('keydown', (e) => {
        const items = autocompleteContainer.querySelectorAll('.SearchAutocomplete-item');
        if (items.length === 0) return;

        if (e.key === 'ArrowDown') {
            e.preventDefault();
            selectedIndex = Math.min(selectedIndex + 1, items.length - 1);
            updateSelection(items, selectedIndex);
        } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            selectedIndex = Math.max(selectedIndex - 1, -1);
            updateSelection(items, selectedIndex);
        } else if (e.key === 'Enter' && selectedIndex >= 0) {
            e.preventDefault();
            const link = items[selectedIndex].querySelector('a');
            if (link) window.location.href = link.href;
        }
    });

    input.addEventListener('blur', () => {
        setTimeout(hideAutocomplete, 150);
    });
}

function fetchSuggestions(query) {
    fetch('/api/search?q=' + encodeURIComponent(query))
        .then(res => res.json())
        .then(results => {
            if (!results || results.length === 0) {
                hideAutocomplete();
                return;
            }
            showAutocomplete(results.slice(0, 8));
        })
        .catch(() => hideAutocomplete());
}

function showAutocomplete(results) {
    if (!autocompleteContainer) return;
    autocompleteContainer.innerHTML = results.map(pkg => {
        const lang = pkg.lang || 'go';
        let langBadge;
        if (lang === 'rust') {
            langBadge = '<span class="SearchAutocomplete-lang SearchAutocomplete-lang--rust">Rust</span>';
        } else if (lang === 'js') {
            langBadge = '<span class="SearchAutocomplete-lang SearchAutocomplete-lang--js">JS</span>';
        } else if (lang === 'python') {
            langBadge = '<span class="SearchAutocomplete-lang SearchAutocomplete-lang--python">Python</span>';
        } else if (lang === 'php') {
            langBadge = '<span class="SearchAutocomplete-lang SearchAutocomplete-lang--php">PHP</span>';
        } else {
            langBadge = '<span class="SearchAutocomplete-lang SearchAutocomplete-lang--go">Go</span>';
        }
        return `
            <div class="SearchAutocomplete-item">
                <a href="/${pkg.import_path}">
                    <span class="SearchAutocomplete-header">
                        ${langBadge}
                        <span class="SearchAutocomplete-path">${pkg.import_path}</span>
                    </span>
                    <span class="SearchAutocomplete-synopsis">${pkg.synopsis || ''}</span>
                </a>
            </div>
        `;
    }).join('');
    autocompleteContainer.style.display = 'block';
}

function hideAutocomplete() {
    if (autocompleteContainer) {
        autocompleteContainer.style.display = 'none';
        autocompleteContainer.innerHTML = '';
    }
}

function updateSelection(items, index) {
    items.forEach((item, i) => {
        item.classList.toggle('selected', i === index);
    });
}

function isInputFocused() {
    const tag = document.activeElement?.tagName;
    return tag === 'INPUT' || tag === 'TEXTAREA' || document.activeElement?.isContentEditable;
}

// Lazy loading for large packages
function initLazyLoading() {
    const INITIAL_SHOW = 10;

    // Apply to functions section
    const functionsSection = document.querySelector('#pkg-functions');
    if (functionsSection) {
        const functions = functionsSection.querySelectorAll('.Documentation-function');
        applyLazyLoad(functions, functionsSection, 'functions');
    }

    // Apply to types section (each type's methods)
    document.querySelectorAll('.Documentation-type').forEach(typeSection => {
        const methods = typeSection.querySelectorAll('.Documentation-function');
        if (methods.length > INITIAL_SHOW) {
            applyLazyLoad(methods, typeSection, 'methods');
        }
    });

    function applyLazyLoad(items, container, label) {
        if (items.length <= INITIAL_SHOW) return;

        const hiddenItems = [];
        items.forEach((item, index) => {
            if (index >= INITIAL_SHOW) {
                item.classList.add('lazy-hidden');
                item.style.display = 'none';
                hiddenItems.push(item);
            }
        });

        const showMoreBtn = document.createElement('button');
        showMoreBtn.className = 'ShowMore-button';
        showMoreBtn.innerHTML = `Show ${hiddenItems.length} more ${label} <span class="ShowMore-icon">▼</span>`;

        let expanded = false;
        showMoreBtn.addEventListener('click', () => {
            expanded = !expanded;
            hiddenItems.forEach(item => {
                item.style.display = expanded ? '' : 'none';
            });
            if (expanded) {
                showMoreBtn.innerHTML = `Show fewer ${label} <span class="ShowMore-icon">▲</span>`;
            } else {
                showMoreBtn.innerHTML = `Show ${hiddenItems.length} more ${label} <span class="ShowMore-icon">▼</span>`;
            }
        });

        // Insert button after the last visible item
        items[INITIAL_SHOW - 1].after(showMoreBtn);
    }
}

// In-page symbol search with type filters
function initSymbolSearch() {
    const searchContainer = document.querySelector('.Package-nav');
    if (!searchContainer) return;

    const navInner = searchContainer.querySelector('.Package-navInner');
    if (!navInner) return;

    // Create search wrapper
    const searchWrapper = document.createElement('div');
    searchWrapper.className = 'SymbolSearch';

    // Create search input
    const searchInput = document.createElement('input');
    searchInput.type = 'text';
    searchInput.placeholder = 'Filter symbols...';
    searchInput.className = 'SymbolSearch-input';

    // Create filter buttons
    const filterWrapper = document.createElement('div');
    filterWrapper.className = 'SymbolSearch-filters';

    const filters = [
        { label: 'All', value: 'all' },
        { label: 'Funcs', value: 'func' },
        { label: 'Types', value: 'type' }
    ];

    let activeFilter = 'all';

    filters.forEach(({ label, value }) => {
        const btn = document.createElement('button');
        btn.textContent = label;
        btn.className = 'SymbolSearch-filter' + (value === 'all' ? ' active' : '');
        btn.dataset.filter = value;
        btn.addEventListener('click', () => {
            activeFilter = value;
            filterWrapper.querySelectorAll('.SymbolSearch-filter').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            applyFilters();
        });
        filterWrapper.appendChild(btn);
    });

    searchWrapper.appendChild(searchInput);
    searchWrapper.appendChild(filterWrapper);
    navInner.insertBefore(searchWrapper, navInner.firstChild);

    const allNavItems = searchContainer.querySelectorAll('.Package-navList li, .Package-navSublist li');
    const allDetails = searchContainer.querySelectorAll('.Package-navDetails');

    function applyFilters() {
        const query = searchInput.value.toLowerCase().trim();

        allNavItems.forEach(item => {
            const link = item.querySelector('a');
            if (!link) return;

            const text = link.textContent.toLowerCase();
            const matchesQuery = !query || text.includes(query);

            let matchesType = true;
            if (activeFilter !== 'all') {
                const itemKind = item.dataset.kind || item.closest('[data-kind]')?.dataset.kind || '';
                matchesType = itemKind === activeFilter;
            }

            item.style.display = (matchesQuery && matchesType) ? '' : 'none';
        });

        // Show parent details if any child matches
        allDetails.forEach(details => {
            const visibleItems = details.querySelectorAll('li:not([style*="display: none"])');
            details.style.display = visibleItems.length > 0 ? '' : 'none';
            if (visibleItems.length > 0) {
                details.open = true;
            }
        });
    }

    searchInput.addEventListener('input', applyFilters);

    // ? key to focus symbol search
    document.addEventListener('keydown', (e) => {
        if (e.key === '?' && !isInputFocused()) {
            e.preventDefault();
            searchInput.focus();
        }
    });
}

// Add copy button styles
const style = document.createElement('style');
style.textContent = `
    .copy-button {
        position: absolute;
        top: 0.5rem;
        right: 0.5rem;
        padding: 0.25rem 0.5rem;
        font-size: 0.75rem;
        color: #abb2bf;
        background: #3e4451;
        border: none;
        border-radius: 0.25rem;
        cursor: pointer;
        opacity: 0;
        transition: opacity 0.2s;
    }
    pre:hover .copy-button {
        opacity: 1;
    }
    .copy-button:hover {
        background: #4b5363;
    }
    .Package-navList a.active {
        color: #007d9c;
        font-weight: 500;
    }
    .SymbolSearch {
        margin-bottom: 1rem;
    }
    .SymbolSearch-input {
        width: 100%;
        padding: 0.5rem;
        margin-bottom: 0.5rem;
        font-size: 0.875rem;
        border: 1px solid var(--color-border);
        border-radius: 0.25rem;
        background: var(--color-background);
        color: var(--color-text);
    }
    .SymbolSearch-input:focus {
        outline: none;
        border-color: var(--color-brand);
    }
    .SymbolSearch-input::placeholder {
        color: var(--color-text-secondary);
    }
    .SymbolSearch-filters {
        display: flex;
        gap: 0.25rem;
    }
    .SymbolSearch-filter {
        flex: 1;
        padding: 0.25rem 0.5rem;
        font-size: 0.75rem;
        border: 1px solid var(--color-border);
        border-radius: 0.25rem;
        background: var(--color-background);
        color: var(--color-text-secondary);
        cursor: pointer;
    }
    .SymbolSearch-filter:hover {
        background: var(--color-background-secondary);
    }
    .SymbolSearch-filter.active {
        background: var(--color-brand);
        color: #fff;
        border-color: var(--color-brand);
    }
    .SearchAutocomplete {
        display: none;
        position: absolute;
        top: 100%;
        left: 0;
        right: 0;
        background: var(--color-background);
        border: 1px solid var(--color-border);
        border-radius: 0.25rem;
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
        z-index: 1000;
        max-height: 400px;
        overflow-y: auto;
    }
    .SearchAutocomplete-item {
        border-bottom: 1px solid var(--color-border);
    }
    .SearchAutocomplete-item:last-child {
        border-bottom: none;
    }
    .SearchAutocomplete-item a {
        display: block;
        padding: 0.75rem 1rem;
        text-decoration: none;
        color: var(--color-text);
    }
    .SearchAutocomplete-item:hover,
    .SearchAutocomplete-item.selected {
        background: var(--color-background-secondary);
    }
    .SearchAutocomplete-header {
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }
    .SearchAutocomplete-lang {
        font-size: 0.65rem;
        font-weight: 600;
        padding: 0.1rem 0.4rem;
        border-radius: 0.25rem;
        text-transform: uppercase;
    }
    .SearchAutocomplete-lang--go {
        background: #00add8;
        color: white;
    }
    .SearchAutocomplete-lang--rust {
        background: #dea584;
        color: #1a1a1a;
    }
    .SearchAutocomplete-lang--js {
        background: #f7df1e;
        color: #1a1a1a;
    }
    .SearchAutocomplete-lang--python {
        background: #3776ab;
        color: white;
    }
    .SearchAutocomplete-lang--php {
        background: #8892bf;
        color: white;
    }
    .SearchAutocomplete-path {
        font-family: var(--font-family-mono);
        font-size: 0.875rem;
        color: var(--color-link);
    }
    .SearchAutocomplete-synopsis {
        display: block;
        font-size: 0.75rem;
        color: var(--color-text-secondary);
        margin-top: 0.25rem;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }
    .ShowMore-button {
        display: block;
        width: 100%;
        padding: 0.75rem 1rem;
        margin: 1rem 0;
        font-size: 0.875rem;
        color: var(--color-link);
        background: var(--color-background-secondary);
        border: 1px dashed var(--color-border);
        border-radius: 0.5rem;
        cursor: pointer;
        text-align: center;
        transition: all 0.2s;
    }
    .ShowMore-button:hover {
        background: var(--color-border);
        border-style: solid;
    }
    .ShowMore-icon {
        font-size: 0.75rem;
        margin-left: 0.25rem;
    }
    .PlaygroundEmbed {
        margin-top: 1rem;
        border: 1px solid var(--color-border);
        border-radius: 0.5rem;
        overflow: hidden;
    }
    .PlaygroundEmbed-controls {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0.5rem 1rem;
        background: var(--color-background-secondary);
        border-bottom: 1px solid var(--color-border);
    }
    .PlaygroundEmbed-open {
        font-size: 0.875rem;
        color: var(--color-link);
    }
    .PlaygroundEmbed-close {
        padding: 0.25rem 0.75rem;
        font-size: 0.75rem;
        color: var(--color-text-secondary);
        background: var(--color-background);
        border: 1px solid var(--color-border);
        border-radius: 0.25rem;
        cursor: pointer;
    }
    .PlaygroundEmbed-close:hover {
        background: var(--color-border);
    }
    .PlaygroundEmbed-iframe {
        width: 100%;
        height: 400px;
        border: none;
        background: #fff;
    }
`;
document.head.appendChild(style);

// Initialize symbol search on DOMContentLoaded
document.addEventListener('DOMContentLoaded', initSymbolSearch);

// Explain code with AI
async function explainCode(btn) {
    const code = btn.getAttribute('data-code');
    if (!code) return;

    // Disable button during request
    btn.disabled = true;
    const originalText = btn.textContent;
    btn.textContent = 'Loading...';

    try {
        const response = await fetch('/api/explain', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ code }),
        });

        if (!response.ok) {
            throw new Error('Failed to get explanation');
        }

        const data = await response.json();

        // Show explanation in modal
        showExplanationModal(code, data.explanation);
    } catch (error) {
        console.error('Error explaining code:', error);
        alert('Failed to generate explanation. AI service may not be available.');
    } finally {
        btn.disabled = false;
        btn.textContent = originalText;
    }
}

// Show explanation modal
function showExplanationModal(code, explanation) {
    // Create modal if it doesn't exist
    let modal = document.getElementById('explanationModal');
    if (!modal) {
        modal = document.createElement('div');
        modal.id = 'explanationModal';
        modal.className = 'ExplanationModal';
        modal.innerHTML = `
            <div class="ExplanationModal-content">
                <div class="ExplanationModal-header">
                    <h3>Code Explanation</h3>
                    <button class="ExplanationModal-close" onclick="closeExplanationModal()">&times;</button>
                </div>
                <div class="ExplanationModal-body">
                    <div class="ExplanationModal-code">
                        <pre><code class="language-go"></code></pre>
                    </div>
                    <div class="ExplanationModal-explanation"></div>
                </div>
            </div>
        `;
        document.body.appendChild(modal);
    }

    // Update content
    modal.querySelector('.ExplanationModal-code code').textContent = code;
    modal.querySelector('.ExplanationModal-explanation').textContent = explanation;

    // Show modal
    modal.style.display = 'flex';

    // Highlight code
    if (window.Prism) {
        Prism.highlightElement(modal.querySelector('.ExplanationModal-code code'));
    }
}

// Close explanation modal
function closeExplanationModal() {
    const modal = document.getElementById('explanationModal');
    if (modal) {
        modal.style.display = 'none';
    }
}

// Close modal on outside click
document.addEventListener('click', function(e) {
    const modal = document.getElementById('explanationModal');
    if (modal && e.target === modal) {
        closeExplanationModal();
    }
});

// Close modal on Escape key
document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') {
        closeExplanationModal();
    }
});
