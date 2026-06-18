import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar header styles', () => {
  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})

describe('AppSidebar external payment entry', () => {
  it('keeps the configured payment shop URL as a direct sidebar link', () => {
    expect(componentSource).toContain("const EXTERNAL_PAYMENT_URL = 'https://pay.ldxp.cn/shop/Y5TXJ2DO'")
    expect(componentSource).toContain("label: t('nav.externalPayment')")
    expect(componentSource).toContain('externalUrl: EXTERNAL_PAYMENT_URL')
    expect(componentSource).toContain(':href="item.externalUrl"')
    expect(componentSource).toContain('target="_blank"')
    expect(componentSource).toContain('rel="noopener noreferrer"')
  })
})

describe('AppSidebar admin token leaderboard entry', () => {
  it('keeps the admin Token leaderboard as an admin-only navigation item', () => {
    expect(componentSource).toContain("path: '/admin/token-leaderboard'")
    expect(componentSource).toContain("label: t('nav.tokenLeaderboard')")
  })
})

describe('AppSidebar admin balance summary entry', () => {
  it('keeps the balance summary as an admin-only navigation item', () => {
    expect(componentSource).toContain("path: '/admin/balance-summary'")
    expect(componentSource).toContain("label: t('nav.balanceSummary')")
  })
})

describe('AppSidebar leaderboard entry', () => {
  it('adds leaderboard to the shared user and admin personal navigation declaration', () => {
    expect(componentSource).toContain("{ path: '/leaderboard', label: t('nav.leaderboard'), icon: ChartIcon }")
    expect(componentSource).toContain('const userNavItems = computed((): NavItem[] => finalizeNav(buildSelfNavItems(true)))')
    expect(componentSource).toContain('const personalNavItems = computed((): NavItem[] => finalizeNav(buildSelfNavItems(false)))')
  })
})
