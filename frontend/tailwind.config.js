/** @type {import('tailwindcss').Config} */
export default {
    content: [
        './index.html',
        './src/**/*.{vue,js,ts,jsx,tsx}',
    ],
    darkMode: 'class',
    theme: {
        extend: {
            // Premium Dark/Light Theme Palette mapped to CSS variables
            colors: {
                background: 'var(--color-bg-base)',
                surface: {
                    50: 'var(--color-surface-50)',
                    100: 'var(--color-surface-100)',
                    200: 'var(--color-surface-200)',
                    300: 'var(--color-surface-300)',
                    400: 'var(--color-surface-400)',
                    500: 'var(--color-surface-500)',
                    600: 'var(--color-surface-600)',
                    700: 'var(--color-surface-700)',
                    800: 'var(--color-surface-800)',
                    900: 'var(--color-surface-900)',
                    950: 'var(--color-surface-950)',
                },
                panel: 'var(--color-bg-surface)',
                elevated: 'var(--color-bg-elevated)',
                inset: 'var(--color-bg-inset)',
                border: 'var(--color-border)',
                'border-strong': 'var(--color-border-strong)',
                fg: {
                    DEFAULT: 'var(--color-text-main)',
                    muted: 'var(--color-text-muted)',
                    subtle: 'var(--color-text-subtle)',
                    faint: 'var(--color-text-faint)',
                    inverse: 'var(--color-text-inverse)',
                },
                primary: {
                    50: '#fdf8f6',
                    100: '#f2e8e5',
                    200: '#eaddd5',
                    300: '#e0cec7',
                    400: '#d2bab0',
                    500: 'var(--color-primary-500)',
                    600: 'var(--color-primary-600)',
                    700: '#975a45',
                    800: '#844d38',
                    900: '#713f2e',
                    950: '#5e3124',
                },
                accent: {
                    500: '#F59E0B',
                    600: '#D97706',
                },
                success: '#10b981',
                warning: '#f59e0b',
                error: '#ef4444',
            },
            animation: {
                'fade-in': 'fadeIn 0.3s ease-in-out',
                'slide-in': 'slideIn 0.3s ease-out',
                'slide-in-from-top': 'slideInFromTop 0.2s ease-out',
                'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
            },
            keyframes: {
                fadeIn: {
                    '0%': { opacity: '0' },
                    '100%': { opacity: '1' },
                },
                slideIn: {
                    '0%': { transform: 'translateX(-100%)' },
                    '100%': { transform: 'translateX(0)' },
                },
            },
        },
        fontFamily: {
            sans: ['Inter', 'Outfit', 'system-ui', 'sans-serif'],
            display: ['Outfit', 'Inter', 'system-ui', 'sans-serif'],
        },
        backgroundImage: {
            'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
            'hero-glow': 'conic-gradient(from 180deg at 50% 50%, #D4A373 0deg, #A98467 180deg, #8D6E63 360deg)',
        },
        boxShadow: {
            'glow': '0 0 20px rgba(212, 163, 115, 0.5)',
            'glass': '0 8px 32px 0 rgba(31, 38, 135, 0.37)',
        },
    },
    plugins: [],
}
