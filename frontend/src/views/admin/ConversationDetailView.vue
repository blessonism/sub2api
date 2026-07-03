<template>
  <AppLayout>
    <div v-if="sessionLoading" class="flex min-h-96 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="!session" class="flex min-h-96 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
      Session not found
    </div>
    <template v-else>
      <!-- Sticky header -->
      <div class="sticky top-0 z-20 -mx-4 -mt-4 mb-6 border-b border-gray-100 bg-white/95 px-4 py-3 backdrop-blur-sm dark:border-dark-700 dark:bg-dark-900/95 md:-mx-6 md:-mt-6 md:px-6 lg:-mx-8 lg:-mt-8 lg:px-8">
        <div class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            class="inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-100"
            @click="router.push('/admin/conversations')"
          >
            <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" />
            </svg>
            {{ t('admin.conversations.backToList') }}
          </button>
          <span class="text-gray-200 dark:text-dark-600">/</span>
          <code class="rounded bg-gray-100 px-1.5 py-0.5 font-mono text-xs text-gray-700 dark:bg-dark-700 dark:text-gray-200">
            {{ shortId(session.session_id) }}
          </code>
          <span :class="pillClass(qualityKind(session.quality_status))" class="rounded px-1.5 py-0.5 text-xs font-medium">
            {{ qualityLabel(session.quality_status) }}
          </span>
          <button
            type="button"
            class="rounded px-1.5 py-0.5 text-xs font-medium transition"
            :class="session.exportable
              ? 'bg-emerald-50 text-emerald-700 hover:bg-emerald-100 dark:bg-emerald-900/30 dark:text-emerald-300'
              : 'bg-gray-100 text-gray-500 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400'"
            @click="toggleSessionExportable"
          >
            {{ session.exportable ? t('admin.conversations.exportable') : t('admin.conversations.notExportable') }}
          </button>

          <!-- Quality dropdown -->
          <div class="relative">
            <button
              type="button"
              class="inline-flex items-center gap-1 rounded border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-600 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-900 dark:text-gray-300"
              @click="openSessionMenu = !openSessionMenu"
            >
              {{ t('admin.conversations.qualityDropdown') }}
              <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
              </svg>
            </button>
            <div
              v-if="openSessionMenu"
              class="absolute left-0 top-full z-30 mt-1 min-w-[140px] rounded-md border border-gray-100 bg-white py-1 shadow-lg dark:border-dark-700 dark:bg-dark-800"
            >
              <button
                v-for="s in qualityStatuses"
                :key="s"
                type="button"
                class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700"
                :class="session.quality_status === s ? 'font-semibold' : ''"
                @click="markSessionQuality(s); openSessionMenu = false"
              >
                {{ qualityLabel(s) }}
              </button>
            </div>
          </div>

          <!-- More actions -->
          <div class="relative ml-auto">
            <button
              type="button"
              class="rounded border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-600 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-900 dark:text-gray-300"
              @click="openMoreMenu = !openMoreMenu"
            >
              {{ t('admin.conversations.moreActions') }}
            </button>
            <div
              v-if="openMoreMenu"
              class="absolute right-0 top-full z-30 mt-1 min-w-[200px] rounded-md border border-gray-100 bg-white p-3 shadow-lg dark:border-dark-700 dark:bg-dark-800"
            >
              <div class="mb-1 text-xs text-gray-400 dark:text-gray-500">{{ t('admin.conversations.mergeSourceIds') }}</div>
              <div class="flex gap-1.5">
                <input
                  v-model="mergeSourceIDsText"
                  class="input min-w-0 flex-1 text-xs"
                  :placeholder="t('admin.conversations.mergeSourceIds')"
                />
                <button class="btn btn-secondary shrink-0 px-2 py-1 text-xs" type="button" @click="mergeIntoSession">
                  {{ t('admin.conversations.merge') }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Session metadata -->
        <div class="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
          <span class="font-medium text-gray-700 dark:text-gray-200">{{ session.user_email || `#${session.user_id}` }}</span>
          <span>key #{{ session.api_key_id }}</span>
          <span class="rounded bg-primary-50 px-1.5 py-0.5 font-mono font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">{{ session.model }}</span>
          <span>{{ session.provider }}</span>
          <span>{{ formatDateShort(session.started_at) }}<span v-if="session.ended_at"> → {{ formatTimeOnly(session.ended_at) }}</span></span>
          <span v-if="sessionDuration">{{ sessionDuration }}</span>
          <span class="text-gray-300 dark:text-dark-600">·</span>
          <span class="tabular-nums">{{ session.turn_count }} turns</span>
          <span class="tabular-nums">{{ formatNumberCompact(session.total_tokens) }} tok</span>
          <span class="tabular-nums">{{ formatCostCompact(session.actual_cost) }}</span>
        </div>
      </div>

      <!-- Turn timeline -->
      <div class="space-y-3">
        <div
          v-for="turn in turns"
          :key="turn.id"
          :data-turn-id="String(turn.id)"
          class="card overflow-hidden"
        >
          <!-- Turn header -->
          <div class="flex flex-wrap items-center gap-2 border-b border-gray-100 bg-gray-50/50 px-4 py-2 dark:border-dark-700 dark:bg-dark-800/30">
            <span class="rounded bg-gray-100 px-1.5 py-0.5 font-mono text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">
              #{{ turn.turn_index }}
            </span>
            <span class="text-xs text-gray-400 dark:text-gray-500">{{ formatDateShort(turn.created_at) }}</span>
            <span class="text-xs text-gray-300 dark:text-dark-600">·</span>
            <span class="tabular-nums text-xs text-gray-500 dark:text-gray-400">{{ formatNumberCompact(turn.total_tokens) }} tok</span>
            <span class="text-xs text-gray-300 dark:text-dark-600">·</span>
            <span class="tabular-nums text-xs text-gray-500 dark:text-gray-400">{{ formatCostCompact(turn.actual_cost) }}</span>
            <span v-if="turn.duplicate_count > 0" class="text-xs text-amber-600 dark:text-amber-400">×{{ turn.duplicate_count }} dup</span>
            <span v-if="turn.truncated" class="text-xs text-amber-600 dark:text-amber-400">trunc</span>
            <span v-if="turn.parse_status === 'failed'" class="text-xs text-rose-500 dark:text-rose-400">parse-err</span>
            <span v-if="turn.client_disconnect" class="text-xs text-gray-400">disconnect</span>
            <span :class="pillClass(qualityKind(turn.quality_status))" class="rounded px-1.5 py-0.5 text-xs">
              {{ qualityLabel(turn.quality_status) }}
            </span>
            <span
              v-if="!turn.exportable"
              class="rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-400 dark:bg-dark-700 dark:text-gray-500"
            >
              {{ t('admin.conversations.notExportable') }}
            </span>

            <!-- Turn ⋯ menu -->
            <div class="relative ml-auto">
              <button
                type="button"
                class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-gray-200"
                @click="toggleTurnMenu(turn.id)"
              >
                <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M12 5a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3zm0 7a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3zm0 7a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3z" />
                </svg>
              </button>
              <div
                v-if="openTurnMenu === turn.id"
                class="absolute right-0 top-full z-30 mt-1 min-w-[160px] rounded-md border border-gray-100 bg-white py-1 shadow-lg dark:border-dark-700 dark:bg-dark-800"
              >
                <button
                  type="button"
                  class="flex w-full items-center px-3 py-1.5 text-left text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700"
                  @click="markTurnQuality(turn, 'clean'); openTurnMenu = null"
                >{{ t('admin.conversations.markClean') }}</button>
                <button
                  type="button"
                  class="flex w-full items-center px-3 py-1.5 text-left text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700"
                  @click="markTurnQuality(turn, 'rejected'); openTurnMenu = null"
                >{{ t('admin.conversations.reject') }}</button>
                <button
                  type="button"
                  class="flex w-full items-center px-3 py-1.5 text-left text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700"
                  @click="toggleTurnExportable(turn); openTurnMenu = null"
                >{{ turn.exportable ? t('admin.conversations.unmark') : t('admin.conversations.markExportable') }}</button>
                <hr class="my-1 border-gray-100 dark:border-dark-700" />
                <button
                  type="button"
                  class="flex w-full items-center px-3 py-1.5 text-left text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700"
                  @click="splitFromTurn(turn.id); openTurnMenu = null"
                >{{ t('admin.conversations.splitHere') }}</button>
                <div class="flex items-center gap-1.5 px-3 py-1.5">
                  <input
                    v-model.number="moveTargets[turn.id]"
                    class="input min-w-0 flex-1 text-xs"
                    type="number"
                    min="1"
                    :placeholder="t('admin.conversations.moveTarget')"
                  />
                  <button
                    class="shrink-0 rounded border border-gray-200 bg-white px-1.5 py-1 text-xs text-gray-600 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-900 dark:text-gray-300"
                    type="button"
                    @click="moveTurnToSession(turn.id)"
                  >{{ t('admin.conversations.move') }}</button>
                </div>
              </div>
            </div>
          </div>

          <!-- Messages area -->
          <div class="p-4">
            <div v-if="turnDetailLoading.has(turn.id)" class="flex justify-center py-6">
              <LoadingSpinner />
            </div>
            <div v-else-if="turnDetailErrors[turn.id]" class="flex items-center gap-2 rounded-md bg-rose-50 px-3 py-2 text-xs text-rose-700 dark:bg-rose-900/20 dark:text-rose-300">
              <span>{{ t('admin.conversations.loadDetailFailed') }}</span>
              <button type="button" class="ml-auto shrink-0 underline hover:no-underline" @click="fetchTurnDetail(turn.id)">
                {{ t('common.retry') }}
              </button>
            </div>
            <template v-else-if="selectedTurnDetails[turn.id]">
              <div class="space-y-2">
                <div
                  v-for="(msg, i) in allMessages(selectedTurnDetails[turn.id])"
                  :key="i"
                >
                  <!-- System message (collapsible) -->
                  <template v-if="String(msg.role) === 'system'">
                    <button
                      type="button"
                      class="flex w-full items-center gap-2 rounded-md bg-gray-100 px-3 py-2 text-left text-xs text-gray-500 hover:bg-gray-150 dark:bg-dark-700 dark:text-gray-400"
                      @click="toggleSystemMsg(turn.id, i)"
                    >
                      <svg
                        class="h-3 w-3 shrink-0 transition-transform"
                        :class="isSystemExpanded(turn.id, i) ? 'rotate-90' : ''"
                        fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"
                      >
                        <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
                      </svg>
                      <span class="font-medium">{{ t('admin.conversations.systemRole') }}</span>
                      <span v-if="!isSystemExpanded(turn.id, i)" class="truncate text-gray-400 dark:text-gray-500">
                        {{ previewText(msg.content) }}
                      </span>
                    </button>
                    <div v-if="isSystemExpanded(turn.id, i)" class="mt-1 rounded-md bg-gray-100 px-3 py-2.5 dark:bg-dark-700">
                      <pre class="whitespace-pre-wrap break-words font-sans text-xs text-gray-700 dark:text-gray-300">{{ getTextContent(msg.content) }}</pre>
                    </div>
                  </template>

                  <!-- Tool use -->
                  <template v-else-if="isToolUseMsg(msg)">
                    <div class="rounded-md border border-purple-100 bg-purple-50/50 dark:border-purple-900/30 dark:bg-purple-900/10">
                      <div class="border-b border-purple-100 px-3 py-1.5 text-xs font-medium text-purple-700 dark:border-purple-900/30 dark:text-purple-300">
                        {{ t('admin.conversations.toolUse') }}
                      </div>
                      <pre class="max-h-64 overflow-auto p-3 font-mono text-xs text-purple-900 dark:text-purple-200">{{ getTextContent(msg.content) }}</pre>
                    </div>
                  </template>

                  <!-- User message -->
                  <template v-else-if="String(msg.role) === 'user'">
                    <div class="rounded-md bg-blue-50 px-3 py-2.5 dark:bg-blue-900/20">
                      <div class="mb-1 text-xs font-semibold text-blue-700 dark:text-blue-300">{{ t('admin.conversations.userRole') }}</div>
                      <pre class="whitespace-pre-wrap break-words font-sans text-xs text-blue-900 dark:text-blue-200">{{ getTextContent(msg.content) }}</pre>
                    </div>
                  </template>

                  <!-- Assistant message -->
                  <template v-else-if="String(msg.role) === 'assistant'">
                    <div class="rounded-md bg-emerald-50 px-3 py-2.5 dark:bg-emerald-900/20">
                      <div class="mb-1 text-xs font-semibold text-emerald-700 dark:text-emerald-300">{{ t('admin.conversations.assistantRole') }}</div>
                      <pre class="whitespace-pre-wrap break-words font-sans text-xs text-emerald-900 dark:text-emerald-200">{{ getTextContent(msg.content) }}</pre>
                    </div>
                  </template>

                  <!-- Other roles -->
                  <template v-else>
                    <div class="rounded-md bg-gray-100 px-3 py-2.5 dark:bg-dark-700">
                      <div class="mb-1 text-xs font-semibold capitalize text-gray-600 dark:text-gray-300">{{ msg.role }}</div>
                      <pre class="whitespace-pre-wrap break-words font-sans text-xs text-gray-800 dark:text-gray-200">{{ getTextContent(msg.content) }}</pre>
                    </div>
                  </template>
                </div>
              </div>
            </template>
            <!-- Placeholder before detail loads -->
            <div v-else class="rounded-md bg-gray-50 px-3 py-2.5 dark:bg-dark-800">
              <pre class="whitespace-pre-wrap break-words font-sans text-xs text-gray-400 dark:text-gray-500">{{ turn.payload_preview || '…' }}</pre>
            </div>
          </div>
        </div>
      </div>

      <!-- Load more / all loaded -->
      <div class="mt-6 flex justify-center">
        <button
          v-if="hasMoreTurns"
          type="button"
          class="btn btn-secondary"
          :disabled="turnsLoading"
          @click="loadMoreTurns"
        >
          {{ turnsLoading ? t('common.loading') : t('admin.conversations.loadMoreTurns') }}
        </button>
        <span v-else-if="turns.length > 0" class="text-xs text-gray-400 dark:text-gray-500">
          {{ t('admin.conversations.allTurnsLoaded', { count: turns.length }) }}
        </span>
      </div>
    </template>

    <!-- Click-outside overlay for menus -->
    <div v-if="openSessionMenu || openMoreMenu || openTurnMenu !== null" class="fixed inset-0 z-20" @click="closeMenus" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import conversationsAPI, {
  type ConversationQualityStatus,
  type ConversationSession,
  type ConversationTurn,
  type ConversationTurnSummary,
} from '@/api/admin/conversations'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const sessionId = computed(() => Number(route.params.id))

const sessionLoading = ref(true)
const turnsLoading = ref(false)
const session = ref<ConversationSession | null>(null)
const turns = ref<ConversationTurnSummary[]>([])
const selectedTurnDetails = reactive<Record<number, ConversationTurn>>({})
const turnDetailLoading = ref(new Set<number>())
const turnDetailErrors = reactive<Record<number, true>>({})
const moveTargets = reactive<Record<number, number | null>>({})
const mergeSourceIDsText = ref('')

const openSessionMenu = ref(false)
const openMoreMenu = ref(false)
const openTurnMenu = ref<number | null>(null)

// system message expand state: key = `${turnId}-${msgIndex}`
const expandedSystemMsgs = ref(new Set<string>())

const turnsPage = ref(1)
const turnsTotalPages = ref(1)
const PAGE_SIZE = 20

const qualityStatuses: ConversationQualityStatus[] = ['clean', 'needs_review', 'rejected', 'unchecked']

const hasMoreTurns = computed(() => turnsPage.value < turnsTotalPages.value)

const sessionDuration = computed(() => {
  if (!session.value?.started_at || !session.value?.ended_at) return ''
  const ms = new Date(session.value.ended_at).getTime() - new Date(session.value.started_at).getTime()
  if (ms <= 0) return ''
  const totalSec = Math.floor(ms / 1000)
  const min = Math.floor(totalSec / 60)
  const sec = totalSec % 60
  return min > 0 ? `${min}m ${sec}s` : `${sec}s`
})

let observer: IntersectionObserver | null = null

onMounted(async () => {
  observer = new IntersectionObserver((entries) => {
    entries.forEach((entry) => {
      if (!entry.isIntersecting) return
      const id = Number(entry.target.getAttribute('data-turn-id'))
      if (!id || selectedTurnDetails[id] || turnDetailLoading.value.has(id) || turnDetailErrors[id]) return
      void fetchTurnDetail(id)
    })
  }, { rootMargin: '300px' })

  sessionLoading.value = true
  try {
    const [s, turnsResp] = await Promise.all([
      conversationsAPI.getSession(sessionId.value),
      conversationsAPI.listSessionTurns(sessionId.value, { page: 1, page_size: PAGE_SIZE }),
    ])
    session.value = s
    turns.value = turnsResp.items
    turnsPage.value = turnsResp.page
    turnsTotalPages.value = Math.ceil(turnsResp.total / PAGE_SIZE)
    await nextTick()
    observeAllTurnCards()
  } finally {
    sessionLoading.value = false
  }
})

onUnmounted(() => observer?.disconnect())

watch(turns, () => {
  nextTick(() => observeAllTurnCards())
})

function observeAllTurnCards() {
  document.querySelectorAll('[data-turn-id]').forEach((el) => observer?.observe(el))
}

async function fetchTurnDetail(id: number) {
  if (selectedTurnDetails[id] || turnDetailLoading.value.has(id)) return
  delete turnDetailErrors[id]
  turnDetailLoading.value = new Set([...turnDetailLoading.value, id])
  try {
    const detail = await conversationsAPI.getTurn(id)
    selectedTurnDetails[id] = detail
  } catch {
    turnDetailErrors[id] = true
  } finally {
    turnDetailLoading.value.delete(id)
    turnDetailLoading.value = new Set(turnDetailLoading.value)
  }
}

async function loadMoreTurns() {
  if (turnsLoading.value || !hasMoreTurns.value) return
  turnsLoading.value = true
  try {
    const resp = await conversationsAPI.listSessionTurns(sessionId.value, {
      page: turnsPage.value + 1,
      page_size: PAGE_SIZE,
    })
    turns.value = [...turns.value, ...resp.items]
    turnsPage.value = resp.page
    turnsTotalPages.value = Math.ceil(resp.total / PAGE_SIZE)
  } finally {
    turnsLoading.value = false
  }
}

async function toggleSessionExportable() {
  if (!session.value) return
  const next = !session.value.exportable
  await conversationsAPI.setSessionExportable(session.value.id, next)
  session.value.exportable = next
}

async function markSessionQuality(status: ConversationQualityStatus) {
  if (!session.value) return
  await conversationsAPI.setSessionQuality(session.value.id, {
    quality_status: status,
    quality_errors: status === 'clean' || status === 'unchecked'
      ? []
      : [{ code: status, message: status, source: 'manual' }],
  })
  session.value.quality_status = status
  if (status === 'rejected') session.value.exportable = false
}

async function markTurnQuality(turn: ConversationTurnSummary, status: ConversationQualityStatus) {
  await conversationsAPI.setTurnQuality(turn.id, {
    quality_status: status,
    quality_errors: status === 'clean' || status === 'unchecked'
      ? []
      : [{ code: status, message: status, source: 'manual' }],
  })
  turn.quality_status = status
  if (status === 'rejected') turn.exportable = false
  if (selectedTurnDetails[turn.id]) {
    selectedTurnDetails[turn.id].quality_status = status
    selectedTurnDetails[turn.id].exportable = turn.exportable
  }
}

async function toggleTurnExportable(turn: ConversationTurnSummary) {
  const next = !turn.exportable
  await conversationsAPI.setTurnExportable(turn.id, next)
  turn.exportable = next
  if (selectedTurnDetails[turn.id]) selectedTurnDetails[turn.id].exportable = next
}

async function splitFromTurn(turnId: number) {
  if (!session.value) return
  await conversationsAPI.splitSession(session.value.id, { turn_id: turnId })
  router.push('/admin/conversations')
}

async function moveTurnToSession(turnId: number) {
  const target = moveTargets[turnId]
  if (!target || target <= 0) return
  await conversationsAPI.moveTurn(turnId, { target_session_id: target })
  turns.value = turns.value.filter((t) => t.id !== turnId)
}

async function mergeIntoSession() {
  if (!session.value) return
  const sourceIDs = parseIDs(mergeSourceIDsText.value).filter((id) => id !== session.value?.id)
  if (sourceIDs.length === 0) return
  await conversationsAPI.mergeSessions({
    target_session_id: session.value.id,
    source_session_ids: sourceIDs,
  })
  mergeSourceIDsText.value = ''
  openMoreMenu.value = false
  // reload turns
  const resp = await conversationsAPI.listSessionTurns(sessionId.value, { page: 1, page_size: PAGE_SIZE })
  turns.value = resp.items
  turnsPage.value = 1
  turnsTotalPages.value = Math.ceil(resp.total / PAGE_SIZE)
  Object.keys(selectedTurnDetails).forEach((k) => delete selectedTurnDetails[Number(k)])
}

function toggleTurnMenu(id: number) {
  openTurnMenu.value = openTurnMenu.value === id ? null : id
  openSessionMenu.value = false
  openMoreMenu.value = false
}

function closeMenus() {
  openSessionMenu.value = false
  openMoreMenu.value = false
  openTurnMenu.value = null
}

function toggleSystemMsg(turnId: number, msgIndex: number) {
  const key = `${turnId}-${msgIndex}`
  if (expandedSystemMsgs.value.has(key)) {
    expandedSystemMsgs.value.delete(key)
  } else {
    expandedSystemMsgs.value.add(key)
  }
  expandedSystemMsgs.value = new Set(expandedSystemMsgs.value)
}

function isSystemExpanded(turnId: number, msgIndex: number) {
  return expandedSystemMsgs.value.has(`${turnId}-${msgIndex}`)
}

function allMessages(detail: ConversationTurn): Array<Record<string, unknown>> {
  return [...detail.request_messages, ...detail.response_messages]
}

function isToolUseMsg(msg: Record<string, unknown>): boolean {
  if (String(msg.role) !== 'assistant') return false
  const content = msg.content
  if (!Array.isArray(content)) return false
  return content.some((b: unknown) => {
    if (b && typeof b === 'object') {
      const block = b as Record<string, unknown>
      return block.type === 'tool_use' || block.type === 'tool_result'
    }
    return false
  })
}

function previewText(content: unknown): string {
  const text = getTextContent(content)
  return text.length > 60 ? `${text.slice(0, 60)}…` : text
}

function getTextContent(content: unknown): string {
  if (content == null) return '-'
  if (typeof content === 'string') return content
  if (Array.isArray(content)) {
    return content.map((block) => {
      if (typeof block === 'string') return block
      if (block && typeof block === 'object') {
        const b = block as Record<string, unknown>
        if (b.type === 'text') return String(b.text ?? '')
        if (b.type === 'tool_use') return `[tool: ${b.name}]\n${JSON.stringify(b.input, null, 2)}`
        if (b.type === 'tool_result') {
          const c = typeof b.content === 'string' ? b.content : JSON.stringify(b.content, null, 2)
          return `[tool_result: ${b.tool_use_id}]\n${c}`
        }
        if (b.type === 'image') return '[image]'
        return JSON.stringify(b, null, 2)
      }
      return String(block)
    }).filter(Boolean).join('\n\n')
  }
  return JSON.stringify(content, null, 2)
}

function parseIDs(value: string): number[] {
  return value
    .split(/[\s,，;；]+/)
    .map((item) => Number(item.trim()))
    .filter((item) => Number.isInteger(item) && item > 0)
}

function shortId(value: string): string {
  return value.length > 18 ? `${value.slice(0, 10)}...${value.slice(-6)}` : value
}

function formatNumberCompact(value: number): string {
  if (!value) return '0'
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`
  if (value >= 10_000) return `${Math.round(value / 1_000)}K`
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}K`
  return String(value)
}

function formatCostCompact(value: number): string {
  if (!value) return '$0'
  if (value >= 1) return `$${value.toFixed(3)}`
  if (value >= 0.001) return `$${value.toFixed(4)}`
  return `$${value.toFixed(6)}`
}

function formatDateShort(value: string): string {
  if (!value) return '-'
  return new Date(value).toLocaleString(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function formatTimeOnly(value: string): string {
  if (!value) return ''
  return new Date(value).toLocaleString(undefined, { hour: '2-digit', minute: '2-digit' })
}

function pillClass(kind: 'success' | 'warn' | 'muted'): string {
  if (kind === 'success') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (kind === 'warn') return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

function qualityKind(status: string): 'success' | 'warn' | 'muted' {
  if (status === 'clean') return 'success'
  if (status === 'needs_review' || status === 'rejected') return 'warn'
  return 'muted'
}

const qualityLabelMap = computed(() => ({
  clean: t('admin.conversations.qualityClean'),
  needs_review: t('admin.conversations.qualityNeedsReview'),
  rejected: t('admin.conversations.qualityRejected'),
  unchecked: t('admin.conversations.qualityUnchecked'),
}))

function qualityLabel(status: string): string {
  return qualityLabelMap.value[status as keyof typeof qualityLabelMap.value] || status
}
</script>
