// Main JavaScript for wikigo

// Theme toggle
(function() {
    const saved = localStorage.getItem('theme');
    if (saved === 'dark' || (!saved && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
        document.documentElement.setAttribute('data-theme', 'dark');
    }
})();

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

    // Create form and submit to playground
    const form = document.createElement('form');
    form.method = 'POST';
    form.action = 'https://go.dev/play/share';
    form.target = '_blank';

    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = 'body';
    input.value = code;
    form.appendChild(input);

    document.body.appendChild(form);

    // Use fetch to get share ID, then open in playground
    fetch('https://go.dev/_/share', {
        method: 'POST',
        body: code,
    })
    .then(response => response.text())
    .then(shareId => {
        window.open('https://go.dev/play/p/' + shareId, '_blank');
    })
    .catch(() => {
        // Fallback: open playground with code in URL (limited)
        window.open('https://go.dev/play/', '_blank');
    })
    .finally(() => {
        form.remove();
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

    // Search form enhancement
    const searchInput = document.querySelector('.SearchForm-input');
    if (searchInput) {
        document.addEventListener('keydown', (e) => {
            // / to focus search
            if (e.key === '/' && !isInputFocused()) {
                e.preventDefault();
                searchInput.focus();
            }
            // Escape to blur search
            if (e.key === 'Escape' && document.activeElement === searchInput) {
                searchInput.blur();
            }
        });
    }
});

function isInputFocused() {
    const tag = document.activeElement?.tagName;
    return tag === 'INPUT' || tag === 'TEXTAREA' || document.activeElement?.isContentEditable;
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
`;
document.head.appendChild(style);
