<template>
  <AppLayout>
    <TablePageLayout>
      <template #actions>
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <!-- Total Requests -->
          <div class="card p-4">
          <div class="flex items-center gap-3">
            <div class="rounded-lg bg-blue-100 p-2 dark:bg-blue-900/30">
              <Icon name="document" size="md" class="text-blue-600 dark:text-blue-400" />
            </div>
            <div>
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('usage.totalRequests') }}
              </p>
              <p class="text-xl font-bold text-gray-900 dark:text-white">
                {{ usageStats?.total_requests?.toLocaleString() || '0' }}
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('usage.inSelectedRange') }}
              </p>
            </div>
          </div>
        </div>

        <!-- Total Tokens -->
        <div class="card p-4">
          <div class="flex items-center gap-3">
            <div class="rounded-lg bg-amber-100 p-2 dark:bg-amber-900/30">
              <Icon name="cube" size="md" class="text-amber-600 dark:text-amber-400" />
            </div>
            <div class="min-w-0 flex-1">
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('usage.totalTokens') }}
              </p>
              <p class="text-xl font-bold text-gray-900 dark:text-white">
                {{ formatTokens(usageStats?.total_tokens || 0) }}
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                <span>{{ t('usage.in') }} {{ formatTokens(usageStats?.total_input_tokens || 0) }}</span>
                <span> · </span>
                <span>{{ t('usage.out') }} {{ formatTokens(usageStats?.total_output_tokens || 0) }}</span>
                <span> · </span>
                <span class="text-sky-600 dark:text-sky-400">{{ t('usage.cacheHit') }} {{ formatTokens(usageStats?.total_cache_read_tokens || 0) }}</span>
                <span> · </span>
                <span class="text-amber-600 dark:text-amber-400">{{ t('usage.cacheCreate') }} {{ formatTokens(usageStats?.total_cache_creation_tokens || 0) }}</span>
              </p>
              <p v-if="usageStats?.calibration_tokens" class="text-xs text-amber-600 dark:text-amber-400">
                {{ t('usage.calibrationTokens') }}: {{ formatSignedTokens(usageStats.calibration_tokens) }}
              </p>
              <p class="text-xs text-gray-400 dark:text-gray-500">
                {{ t('usage.cacheHitRate') }}:
                <template v-if="cacheStats.totalInput > 0">
                  <span class="text-sky-600 dark:text-sky-400">{{ formatTokens(cacheStats.cacheRead) }}</span>
                  <span class="text-gray-400">/</span>
                  <span class="text-gray-600 dark:text-gray-300">{{ formatTokens(cacheStats.totalInput) }}</span>
                  <span class="ml-1">{{ cacheStats.ratePercent }}</span>
                </template>
                <template v-else>-</template>
              </p>
            </div>
          </div>
        </div>

        <!-- Total Cost -->
        <div class="card p-4">
          <div class="flex items-center gap-3">
            <div class="rounded-lg bg-green-100 p-2 dark:bg-green-900/30">
              <Icon name="dollar" size="md" class="text-green-600 dark:text-green-400" />
            </div>
            <div class="min-w-0 flex-1">
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('usage.totalCost') }}
              </p>
              <p class="text-xl font-bold text-green-600 dark:text-green-400">
                ${{ (usageStats?.total_actual_cost || 0).toFixed(4) }}
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('usage.actualCost') }} /
                <span class="line-through">${{ (usageStats?.total_cost || 0).toFixed(4) }}</span>
                {{ t('usage.standardCost') }}
              </p>
            </div>
          </div>
        </div>

        <!-- Average Duration -->
        <div class="card p-4">
          <div class="flex items-center gap-3">
            <div class="rounded-lg bg-purple-100 p-2 dark:bg-purple-900/30">
              <Icon name="clock" size="md" class="text-purple-600 dark:text-purple-400" />
            </div>
            <div>
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('usage.avgDuration') }}
              </p>
              <p class="text-xl font-bold text-gray-900 dark:text-white">
                {{ formatDuration(usageStats?.average_duration_ms || 0) }}
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('usage.perRequest') }}</p>
            </div>
          </div>
        </div>
        </div>
      </template>

      <template #filters>
        <div class="card">
          <div class="px-6 py-4">
          <div class="flex flex-wrap items-end gap-4">
            <!-- 管理员代看用户筛选 -->
            <div v-if="isAdmin" class="min-w-[300px]">
              <label class="input-label">{{ t('usage.adminUserFilter') }}</label>
              <AdminUserSearchPicker
                :users="adminUsers"
                :selected-user="selectedAdminSearchUser"
                :loading="loadingAdminUsers"
                :placeholder="t('usage.adminSearchUserPlaceholder')"
                :search-label="t('common.search')"
                :loading-label="t('usage.adminSearchingUsers')"
                :results-label="t('usage.adminUserSearchResults')"
                :empty-label="t('usage.adminNoUsersFound')"
                :deleted-label="t('usage.adminDeletedUser')"
                :clear-label="t('usage.adminClearSelectedUser')"
                @search="searchAdminUsers"
                @select="handleAdminUserSelect"
                @clear="clearAdminUserSelection"
              />
            </div>

            <!-- API Key Filter -->
            <div class="min-w-[180px]">
              <label class="input-label">{{ t('usage.apiKeyFilter') }}</label>
              <Select
                v-model="filters.api_key_id"
                :options="apiKeyOptions"
                :placeholder="t('usage.allApiKeys')"
                @change="applyFilters"
              />
            </div>

            <!-- Date Range Filter -->
            <div>
              <label class="input-label">{{ t('usage.timeRange') }}</label>
              <DateRangePicker
                v-model:start-date="startDate"
                v-model:end-date="endDate"
                @change="onDateRangeChange"
              />
            </div>

            <!-- Actions -->
            <div class="ml-auto flex items-center gap-3">
              <button
                v-if="isAdmin"
                @click="openCalibrationDialog"
                :disabled="!canCalibrateSelectedAdminUser"
                class="btn btn-secondary"
                :title="calibrationButtonTitle"
              >
                {{ t('usage.adminCalibration') }}
              </button>
              <button @click="applyFilters" :disabled="loading" class="btn btn-secondary">
                {{ t('common.refresh') }}
              </button>
              <button @click="resetFilters" class="btn btn-secondary">
                {{ t('common.reset') }}
              </button>
            </div>
          </div>
        </div>
        </div>
      </template>

      <template #table>
        <!-- Tab 切换栏 -->
        <div v-if="errorViewEnabled" class="mb-0 flex gap-2 border-b border-gray-200 px-4 pt-3 dark:border-dark-700">
          <button class="tab" :class="{ 'tab-active': activeTab === 'usage' }" @click="activeTab = 'usage'">
            {{ t('usage.tabs.usage') }}
          </button>
          <button class="tab" :class="{ 'tab-active': activeTab === 'errors' }" @click="switchToErrors">
            {{ t('usage.tabs.errors') }}
          </button>
        </div>

        <!-- 用量明细表 -->
        <!-- flex 链让 DataTable 根 .table-wrapper(flex:1)拿到有界高度以启用内部滚动。
             虚拟化器测高 race 导致的概率空白,已在 DataTable 内用「就绪门控 + initialRect 兜底」根治。 -->
        <div v-show="activeTab === 'usage'" class="flex min-h-0 flex-1 flex-col">
          <DataTable
          :columns="columns"
          :data="usageLogs"
          :loading="loading"
          :server-side-sort="true"
          :estimate-row-height="88"
          :overscan="12"
          default-sort-key="created_at"
          default-sort-order="desc"
          @sort="handleSort"
        >
          <template #cell-api_key="{ row }">
            <span class="text-sm text-gray-900 dark:text-white">{{
              row.api_key?.name || '-'
            }}</span>
          </template>

          <template #cell-model="{ value }">
            <span class="font-medium text-gray-900 dark:text-white">{{ value }}</span>
          </template>

          <template #cell-reasoning_effort="{ row }">
            <span class="text-sm text-gray-900 dark:text-white">
              {{ formatReasoningEffort(row.reasoning_effort) }}
            </span>
          </template>

          <template #cell-endpoint="{ row }">
            <span class="text-sm text-gray-600 dark:text-gray-300 block max-w-[320px] whitespace-normal break-all">
              {{ formatUsageEndpoints(row) }}
            </span>
          </template>

          <template #cell-stream="{ row }">
            <span
              class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium"
              :class="getRequestTypeBadgeClass(row)"
            >
              {{ getRequestTypeLabel(row) }}
            </span>
          </template>

          <template #cell-billing_mode="{ row }">
            <span class="inline-flex items-center rounded px-1.5 py-0.5 text-xs font-medium"
                  :class="getBillingModeBadgeClass(getDisplayBillingMode(row))">
              {{ getBillingModeLabel(getDisplayBillingMode(row), t) }}
            </span>
          </template>

          <template #cell-tokens="{ row }">
            <!-- 图片生成请求 -->
            <div v-if="isImageUsage(row)" class="flex items-center gap-1.5">
              <svg
                class="h-4 w-4 text-indigo-500"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
                />
              </svg>
              <span class="font-medium text-gray-900 dark:text-white">{{ row.image_count }}{{ t('usage.imageUnit') }}</span>
              <span class="text-gray-400">({{ formatImageBillingSize(row, t) }})</span>
            </div>
            <!-- Token 请求 -->
            <div v-else class="flex items-center gap-1.5">
              <div class="space-y-1.5 text-sm">
                <!-- Input / Output Tokens -->
                <div class="flex items-center gap-2">
                  <!-- Input -->
                  <div class="inline-flex items-center gap-1">
                    <Icon name="arrowDown" size="sm" class="text-emerald-500" />
                    <span class="font-medium text-gray-900 dark:text-white">{{
                      (row.input_tokens ?? 0).toLocaleString()
                    }}</span>
                  </div>
                  <!-- Output -->
                  <div class="inline-flex items-center gap-1">
                    <Icon name="arrowUp" size="sm" class="text-violet-500" />
                    <span class="font-medium text-gray-900 dark:text-white">{{
                      (row.output_tokens ?? 0).toLocaleString()
                    }}</span>
                  </div>
                </div>
                <!-- Cache Tokens (Read + Write) -->
                <div
                  v-if="row.cache_read_tokens > 0 || row.cache_creation_tokens > 0"
                  class="flex items-center gap-2"
                >
                  <!-- Cache Read -->
                  <div v-if="row.cache_read_tokens > 0" class="inline-flex items-center gap-1">
                    <Icon name="inbox" size="sm" class="text-sky-500" />
                    <span class="font-medium text-sky-600 dark:text-sky-400">{{
                      formatCacheTokens(row.cache_read_tokens)
                    }}</span>
                  </div>
                  <!-- Cache Write -->
                  <div v-if="row.cache_creation_tokens > 0" class="inline-flex items-center gap-1">
                    <Icon name="edit" size="sm" class="text-amber-500" />
                    <span class="font-medium text-amber-600 dark:text-amber-400">{{
                      formatCacheTokens(row.cache_creation_tokens)
                    }}</span>
                    <span v-if="row.cache_creation_1h_tokens > 0" class="inline-flex items-center rounded px-1 py-px text-[10px] font-medium leading-tight bg-orange-100 text-orange-600 ring-1 ring-inset ring-orange-200 dark:bg-orange-500/20 dark:text-orange-400 dark:ring-orange-500/30">1h</span>
                    <span v-if="row.cache_ttl_overridden" :title="t('usage.cacheTtlOverriddenHint')" class="inline-flex items-center rounded px-1 py-px text-[10px] font-medium leading-tight bg-rose-100 text-rose-600 ring-1 ring-inset ring-rose-200 dark:bg-rose-500/20 dark:text-rose-400 dark:ring-rose-500/30 cursor-help">R</span>
                  </div>
                </div>
                <div v-if="hasImageOutputTokens(row)" class="flex items-center gap-2">
                  <div class="inline-flex items-center gap-1">
                    <svg class="h-3.5 w-3.5 text-pink-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" /></svg>
                    <span class="font-medium text-pink-600 dark:text-pink-400">{{ row.image_output_tokens.toLocaleString() }}</span>
                  </div>
                </div>
              </div>
              <!-- Token Detail Tooltip -->
              <div
                class="group relative"
                @mouseenter="showTokenTooltip($event, row)"
                @mouseleave="hideTokenTooltip"
              >
                <div
                  class="flex h-4 w-4 cursor-help items-center justify-center rounded-full bg-gray-100 transition-colors group-hover:bg-blue-100 dark:bg-gray-700 dark:group-hover:bg-blue-900/50"
                >
                  <Icon
                    name="infoCircle"
                    size="xs"
                    class="text-gray-400 group-hover:text-blue-500 dark:text-gray-500 dark:group-hover:text-blue-400"
                  />
                </div>
              </div>
            </div>
          </template>

          <template #cell-cost="{ row }">
            <div class="flex items-center gap-1.5 text-sm">
              <span class="font-medium text-green-600 dark:text-green-400">
                ${{ (row.actual_cost ?? 0).toFixed(6) }}
              </span>
              <!-- Cost Detail Tooltip -->
              <div
                class="group relative"
                @mouseenter="showTooltip($event, row)"
                @mouseleave="hideTooltip"
              >
                <div
                  class="flex h-4 w-4 cursor-help items-center justify-center rounded-full bg-gray-100 transition-colors group-hover:bg-blue-100 dark:bg-gray-700 dark:group-hover:bg-blue-900/50"
                >
                  <Icon
                    name="infoCircle"
                    size="xs"
                    class="text-gray-400 group-hover:text-blue-500 dark:text-gray-500 dark:group-hover:text-blue-400"
                  />
                </div>
              </div>
            </div>
          </template>

          <template #cell-latency="{ row }">
            <div class="flex items-stretch gap-2" data-testid="usage-latency-cell">
              <span
                class="w-1 shrink-0 rounded-full"
                :class="row.first_token_ms != null
                  ? ['bg-gradient-to-b from-40% to-60%', LATENCY_BAR_FROM_CLASSES[firstTokenSeverity(row.first_token_ms)], LATENCY_BAR_TO_CLASSES[durationSeverity(row.duration_ms ?? 0)]]
                  : LATENCY_BAR_CLASSES[durationSeverity(row.duration_ms ?? 0)]"
                aria-hidden="true"
              ></span>
              <div class="grid grid-cols-[max-content_max-content] items-baseline gap-x-2 gap-y-0.5 text-xs">
                <span class="text-gray-400 dark:text-gray-500">{{ t('usage.latencyFirstToken') }}</span>
                <span
                  v-if="row.first_token_ms != null"
                  class="font-medium tabular-nums"
                  :class="LATENCY_TEXT_CLASSES[firstTokenSeverity(row.first_token_ms)]"
                >{{ formatDuration(row.first_token_ms) }}</span>
                <span v-else class="text-gray-400 dark:text-gray-500">-</span>
                <span class="text-gray-400 dark:text-gray-500">{{ t('usage.latencyDuration') }}</span>
                <span
                  class="font-medium tabular-nums"
                  :class="LATENCY_TEXT_CLASSES[durationSeverity(row.duration_ms ?? 0)]"
                >{{ formatDuration(row.duration_ms) }}</span>
              </div>
            </div>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-600 dark:text-gray-400">{{
              formatDateTime(value)
            }}</span>
          </template>

          <template #cell-user_agent="{ row }">
            <span v-if="row.user_agent" class="text-sm text-gray-600 dark:text-gray-400 block max-w-[320px] whitespace-normal break-all" :title="row.user_agent">{{ formatUserAgent(row.user_agent) }}</span>
            <span v-else class="text-sm text-gray-400 dark:text-gray-500">-</span>
          </template>

          <template #empty>
            <EmptyState :message="t('usage.noRecords')" />
          </template>
        </DataTable>
        </div>

        <!-- 错误请求表 -->
        <div v-if="errorViewEnabled" v-show="activeTab === 'errors'" class="flex min-h-0 flex-1 flex-col">
          <UserErrorRequestsTable
            :rows="errorRows"
            :total="errorTotal"
            :loading="errorLoading"
            :page="errorPage"
            :page-size="errorPageSize"
            :api-keys="apiKeys"
            @filter="onErrorFilter"
            @update:page="onErrorPage"
            @update:pageSize="onErrorPageSize"
          />
        </div>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0 && activeTab === 'usage'"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>
  </AppLayout>

  <BaseDialog
    v-if="isAdmin"
    :show="calibrationDialogVisible"
    :title="t('usage.adminCalibrationTitle')"
    width="wide"
    @close="closeCalibrationDialog"
  >
    <div class="space-y-5">
      <div class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
        <div class="font-medium">{{ selectedAdminUserLabel }}</div>
        <div class="mt-1 text-xs text-amber-700 dark:text-amber-300">
          {{ t('usage.adminCalibrationAdminOnlyHint') }}
        </div>
        <div
          v-if="calibrationForm.consumptionEnabled && calibrationTokenCurrentTotal <= 0"
          class="mt-1 text-xs text-amber-700 dark:text-amber-300"
        >
          {{ t('usage.adminConsumptionNoTokenAllocationHint') }}
        </div>
      </div>

      <div class="grid gap-3 sm:grid-cols-2">
        <div>
          <label class="input-label">{{ t('usage.adminTokenRangeStart') }}</label>
          <input
            v-model="calibrationForm.tokenStartDate"
            type="date"
            class="input w-full"
            :disabled="!calibrationForm.tokenEnabled && !calibrationForm.consumptionEnabled"
          />
        </div>
        <div>
          <label class="input-label">{{ t('usage.adminTokenRangeEnd') }}</label>
          <input
            v-model="calibrationForm.tokenEndDate"
            type="date"
            class="input w-full"
            :disabled="!calibrationForm.tokenEnabled && !calibrationForm.consumptionEnabled"
          />
        </div>
      </div>

      <div class="grid gap-4 lg:grid-cols-3">
        <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <label class="flex items-center gap-2">
            <input v-model="calibrationForm.tokenEnabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
            <span class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('usage.adminTokenCalibration') }}</span>
          </label>

          <div class="mt-4 space-y-4" :class="{ 'opacity-50': !calibrationForm.tokenEnabled }">
            <div>
              <label class="input-label">{{ t('usage.adminCalibrationMode') }}</label>
              <Select
                v-model="calibrationForm.tokenMode"
                :options="calibrationModeOptions"
                :disabled="!calibrationForm.tokenEnabled"
              />
            </div>
            <div>
              <label class="input-label">{{ tokenValueLabel }}</label>
              <input
                v-model="calibrationForm.tokenValue"
                type="number"
                step="1"
                class="input w-full"
                :disabled="!calibrationForm.tokenEnabled"
                :placeholder="t('usage.adminTokenValuePlaceholder')"
              />
            </div>
            <div class="rounded-lg bg-gray-50 px-3 py-2 text-xs text-gray-600 dark:bg-dark-800 dark:text-dark-300">
              <div class="flex justify-between gap-4">
                <span>{{ t('usage.adminCurrentRangeTokens') }}</span>
                <span class="font-medium text-gray-900 dark:text-white">{{ formatTokens(calibrationTokenCurrentTotal) }}</span>
              </div>
              <div class="mt-1 flex justify-between gap-4">
                <span>{{ t('usage.adminPreviewDelta') }}</span>
                <span class="font-medium" :class="calibrationTokenDelta >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'">
                  {{ formatSignedInteger(calibrationTokenDelta) }}
                </span>
              </div>
            </div>
          </div>
        </section>

        <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <label class="flex items-center gap-2">
            <input v-model="calibrationForm.balanceEnabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
            <span class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('usage.adminBalanceCalibration') }}</span>
          </label>

          <div class="mt-4 space-y-4" :class="{ 'opacity-50': !calibrationForm.balanceEnabled }">
            <div>
              <label class="input-label">{{ t('usage.adminCalibrationMode') }}</label>
              <Select
                v-model="calibrationForm.balanceMode"
                :options="calibrationModeOptions"
                :disabled="!calibrationForm.balanceEnabled"
              />
            </div>
            <div>
              <label class="input-label">{{ balanceValueLabel }}</label>
              <input
                v-model="calibrationForm.balanceValue"
                type="number"
                step="0.000001"
                class="input w-full"
                :disabled="!calibrationForm.balanceEnabled"
                :placeholder="t('usage.adminBalanceValuePlaceholder')"
              />
            </div>
            <div class="rounded-lg bg-gray-50 px-3 py-2 text-xs text-gray-600 dark:bg-dark-800 dark:text-dark-300">
              <div class="flex justify-between gap-4">
                <span>{{ t('usage.adminCurrentBalance') }}</span>
                <span class="font-medium text-gray-900 dark:text-white">${{ calibrationCurrentBalance.toFixed(6) }}</span>
              </div>
              <div class="mt-1 flex justify-between gap-4">
                <span>{{ t('usage.adminPreviewDelta') }}</span>
                <span class="font-medium" :class="calibrationBalanceDelta >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'">
                  {{ formatSignedMoney(calibrationBalanceDelta) }}
                </span>
              </div>
            </div>
          </div>
        </section>

        <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <label class="flex items-center gap-2">
            <input v-model="calibrationForm.consumptionEnabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
            <span class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('usage.adminConsumptionCalibration') }}</span>
          </label>

          <div class="mt-4 space-y-4" :class="{ 'opacity-50': !calibrationForm.consumptionEnabled }">
            <div>
              <label class="input-label">{{ t('usage.adminCalibrationMode') }}</label>
              <Select
                v-model="calibrationForm.consumptionMode"
                :options="calibrationModeOptions"
                :disabled="!calibrationForm.consumptionEnabled"
              />
            </div>
            <div>
              <label class="input-label">{{ consumptionValueLabel }}</label>
              <input
                v-model="calibrationForm.consumptionValue"
                type="number"
                step="0.000001"
                class="input w-full"
                :disabled="!calibrationForm.consumptionEnabled"
                :placeholder="t('usage.adminConsumptionValuePlaceholder')"
              />
            </div>
            <div class="rounded-lg bg-gray-50 px-3 py-2 text-xs text-gray-600 dark:bg-dark-800 dark:text-dark-300">
              <div class="flex justify-between gap-4">
                <span>{{ t('usage.adminCurrentRangeConsumption') }}</span>
                <span class="font-medium text-gray-900 dark:text-white">${{ calibrationConsumptionCurrentTotal.toFixed(6) }}</span>
              </div>
              <div class="mt-1 flex justify-between gap-4">
                <span>{{ t('usage.adminConsumptionPreviewDelta') }}</span>
                <span class="font-medium" :class="calibrationConsumptionDelta >= 0 ? 'text-rose-600 dark:text-rose-400' : 'text-emerald-600 dark:text-emerald-400'">
                  {{ formatSignedMoney(calibrationConsumptionDelta) }}
                </span>
              </div>
              <div class="mt-1 flex justify-between gap-4">
                <span>{{ t('usage.adminWalletImpact') }}</span>
                <span class="font-medium" :class="calibrationConsumptionDelta <= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'">
                  {{ formatSignedMoney(-calibrationConsumptionDelta) }}
                </span>
              </div>
            </div>
          </div>
        </section>
      </div>

      <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('usage.adminCalibrationHistory') }}</h4>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="loadingCalibrations" @click="loadCalibrationHistory">
            {{ t('common.refresh') }}
          </button>
        </div>
        <div v-if="loadingCalibrations" class="text-sm text-gray-500 dark:text-dark-300">
          {{ t('common.loading') }}
        </div>
        <div v-else-if="calibrationHistory.length === 0" class="text-sm text-gray-500 dark:text-dark-300">
          {{ t('usage.adminNoCalibrationHistory') }}
        </div>
        <div v-else class="space-y-2">
          <div
            v-for="item in calibrationHistory"
            :key="item.id"
            class="rounded-md bg-gray-50 px-3 py-2 text-xs text-gray-700 dark:bg-dark-800 dark:text-dark-200"
          >
            <div class="flex flex-wrap items-center justify-between gap-2">
              <span class="font-medium">{{ formatCalibrationSummary(item) }}</span>
              <div class="flex items-center gap-2 text-gray-500 dark:text-dark-400">
                <span v-if="item.revoked_at" class="text-rose-600 dark:text-rose-400">
                  {{ t('usage.adminCalibrationRevoked') }} · {{ formatDateTime(item.revoked_at) }}
                </span>
                <span v-else>{{ formatDateTime(item.created_at) }}</span>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="Boolean(item.revoked_at) || revokingCalibrationID === item.id"
                  @click="revokeCalibration(item)"
                >
                  {{ item.revoked_at ? t('usage.adminCalibrationRevoked') : revokingCalibrationID === item.id ? t('usage.adminCalibrationRevoking') : t('usage.adminCalibrationRevoke') }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="closeCalibrationDialog">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="submittingCalibration" @click="submitCalibration">
          {{ submittingCalibration ? t('usage.adminCalibrationSubmitting') : t('usage.adminCalibrationSubmit') }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <!-- Token Tooltip Portal -->
  <Teleport to="body">
    <div
      v-if="tokenTooltipVisible"
      class="fixed z-[9999] pointer-events-none -translate-y-1/2"
      :style="{
        left: tokenTooltipPosition.x + 'px',
        top: tokenTooltipPosition.y + 'px'
      }"
    >
      <div
        class="whitespace-nowrap rounded-lg border border-gray-700 bg-gray-900 px-3 py-2.5 text-xs text-white shadow-xl dark:border-gray-600 dark:bg-gray-800"
      >
        <div class="space-y-1.5">
          <!-- Token Breakdown -->
          <div>
            <div class="text-xs font-semibold text-gray-300 mb-1">{{ t('usage.tokenDetails') }}</div>
            <div v-if="tokenTooltipData && tokenTooltipData.input_tokens > 0" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('admin.usage.inputTokens') }}</span>
              <span class="font-medium text-white">{{ tokenTooltipData.input_tokens.toLocaleString() }}</span>
            </div>
            <div v-if="tokenTooltipData && tokenTooltipData.output_tokens > 0 && !hasImageOutputTokens(tokenTooltipData)" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('admin.usage.outputTokens') }}</span>
              <span class="font-medium text-white">{{ tokenTooltipData.output_tokens.toLocaleString() }}</span>
            </div>
            <div v-if="tokenTooltipData && hasImageOutputTokens(tokenTooltipData) && textOutputTokens(tokenTooltipData) > 0" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('admin.usage.outputTokens') }}</span>
              <span class="font-medium text-white">{{ textOutputTokens(tokenTooltipData).toLocaleString() }}</span>
            </div>
            <div v-if="tokenTooltipData && hasImageOutputTokens(tokenTooltipData)" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('usage.imageOutputTokens') }}</span>
              <span class="font-medium text-pink-300">{{ tokenTooltipData.image_output_tokens.toLocaleString() }}</span>
            </div>
            <div v-if="tokenTooltipData && tokenTooltipData.cache_creation_tokens > 0">
              <!-- 有 5m/1h 明细时，展开显示 -->
              <template v-if="tokenTooltipData.cache_creation_5m_tokens > 0 || tokenTooltipData.cache_creation_1h_tokens > 0">
                <div v-if="tokenTooltipData.cache_creation_5m_tokens > 0" class="flex items-center justify-between gap-4">
                  <span class="text-gray-400 flex items-center gap-1.5">
                    {{ t('admin.usage.cacheCreation5mTokens') }}
                    <span class="inline-flex items-center rounded px-1 py-px text-[10px] font-medium leading-tight bg-amber-500/20 text-amber-400 ring-1 ring-inset ring-amber-500/30">5m</span>
                  </span>
                  <span class="font-medium text-white">{{ tokenTooltipData.cache_creation_5m_tokens.toLocaleString() }}</span>
                </div>
                <div v-if="tokenTooltipData.cache_creation_1h_tokens > 0" class="flex items-center justify-between gap-4">
                  <span class="text-gray-400 flex items-center gap-1.5">
                    {{ t('admin.usage.cacheCreation1hTokens') }}
                    <span class="inline-flex items-center rounded px-1 py-px text-[10px] font-medium leading-tight bg-orange-500/20 text-orange-400 ring-1 ring-inset ring-orange-500/30">1h</span>
                  </span>
                  <span class="font-medium text-white">{{ tokenTooltipData.cache_creation_1h_tokens.toLocaleString() }}</span>
                </div>
              </template>
              <!-- 无明细时，只显示聚合值 -->
              <div v-else class="flex items-center justify-between gap-4">
                <span class="text-gray-400">{{ t('admin.usage.cacheCreationTokens') }}</span>
                <span class="font-medium text-white">{{ tokenTooltipData.cache_creation_tokens.toLocaleString() }}</span>
              </div>
            </div>
            <div v-if="tokenTooltipData && tokenTooltipData.cache_ttl_overridden" class="flex items-center justify-between gap-4">
              <span class="text-gray-400 flex items-center gap-1.5">
                {{ t('usage.cacheTtlOverriddenLabel') }}
                <span class="inline-flex items-center rounded px-1 py-px text-[10px] font-medium leading-tight bg-rose-500/20 text-rose-400 ring-1 ring-inset ring-rose-500/30">R-{{ tokenTooltipData.cache_creation_1h_tokens > 0 ? '5m' : '1H' }}</span>
              </span>
              <span class="font-medium text-rose-400">{{ tokenTooltipData.cache_creation_1h_tokens > 0 ? t('usage.cacheTtlOverridden1h') : t('usage.cacheTtlOverridden5m') }}</span>
            </div>
            <div v-if="tokenTooltipData && tokenTooltipData.cache_read_tokens > 0" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('admin.usage.cacheReadTokens') }}</span>
              <span class="font-medium text-white">{{ tokenTooltipData.cache_read_tokens.toLocaleString() }}</span>
            </div>
          </div>
          <!-- Total -->
          <div class="flex items-center justify-between gap-6 border-t border-gray-700 pt-1.5">
            <span class="text-gray-400">{{ t('usage.totalTokens') }}</span>
            <span class="font-semibold text-blue-400">{{ ((tokenTooltipData?.input_tokens || 0) + (tokenTooltipData?.output_tokens || 0) + (tokenTooltipData?.cache_creation_tokens || 0) + (tokenTooltipData?.cache_read_tokens || 0)).toLocaleString() }}</span>
          </div>
        </div>
        <!-- Tooltip Arrow (left side) -->
        <div
          class="absolute right-full top-1/2 h-0 w-0 -translate-y-1/2 border-b-[6px] border-r-[6px] border-t-[6px] border-b-transparent border-r-gray-900 border-t-transparent dark:border-r-gray-800"
        ></div>
      </div>
    </div>
  </Teleport>

  <!-- Tooltip Portal -->
  <Teleport to="body">
    <div
      v-if="tooltipVisible"
      class="fixed z-[9999] pointer-events-none -translate-y-1/2"
      :style="{
        left: tooltipPosition.x + 'px',
        top: tooltipPosition.y + 'px'
      }"
    >
      <div
        class="whitespace-nowrap rounded-lg border border-gray-700 bg-gray-900 px-3 py-2.5 text-xs text-white shadow-xl dark:border-gray-600 dark:bg-gray-800"
      >
        <div class="space-y-1.5">
          <!-- Cost Breakdown -->
          <div class="mb-2 border-b border-gray-700 pb-1.5">
            <div class="text-xs font-semibold text-gray-300 mb-1">{{ t('usage.costDetails') }}</div>
            <div v-if="tooltipData && tooltipData.input_cost > 0" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('admin.usage.inputCost') }}</span>
              <span class="font-medium text-white">${{ tooltipData.input_cost.toFixed(6) }}</span>
            </div>
            <div v-if="tooltipData && tooltipData.output_cost > 0" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('admin.usage.outputCost') }}</span>
              <span class="font-medium text-white">${{ tooltipData.output_cost.toFixed(6) }}</span>
            </div>
            <div v-if="tooltipData && hasImageOutputCost(tooltipData)" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('usage.imageOutputCost') }}</span>
              <span class="font-medium text-pink-300">${{ tooltipData.image_output_cost.toFixed(6) }}</span>
            </div>
            <!-- Token billing: show unit prices per 1M tokens -->
            <template v-if="tooltipData && (!getDisplayBillingMode(tooltipData) || getDisplayBillingMode(tooltipData) === BILLING_MODE_TOKEN)">
              <div v-if="tooltipData && tooltipData.input_tokens > 0" class="flex items-center justify-between gap-4">
                <span class="text-gray-400">{{ t('usage.inputTokenPrice') }}</span>
                <span class="font-medium text-sky-300">{{ formatTokenPricePerMillion(tooltipData.input_cost, tooltipData.input_tokens) }} {{ t('usage.perMillionTokens') }}</span>
              </div>
              <div v-if="tooltipData && tooltipData.output_cost > 0 && textOutputTokens(tooltipData) > 0" class="flex items-center justify-between gap-4">
                <span class="text-gray-400">{{ t('usage.outputTokenPrice') }}</span>
                <span class="font-medium text-violet-300">{{ formatTokenPricePerMillion(tooltipData.output_cost, textOutputTokens(tooltipData)) }} {{ t('usage.perMillionTokens') }}</span>
              </div>
              <div v-if="tooltipData && hasImageOutputTokens(tooltipData)" class="flex items-center justify-between gap-4">
                <span class="text-gray-400">{{ t('usage.imageOutputTokenPrice') }}</span>
                <span class="font-medium text-pink-300">{{ formatTokenPricePerMillion(tooltipData.image_output_cost ?? 0, tooltipData.image_output_tokens) }} {{ t('usage.perMillionTokens') }}</span>
              </div>
            </template>
            <!-- Per-image billing: show image metadata and unit price -->
            <template v-else-if="tooltipData && isImageUsage(tooltipData)">
              <div class="flex items-center justify-between gap-4">
                <span class="text-gray-400">{{ t('usage.imageCount') }}</span>
                <span class="font-medium text-white">{{ tooltipData.image_count }}{{ t('usage.imageUnit') }}</span>
              </div>
              <div class="flex items-center justify-between gap-4">
                <span class="text-gray-400">{{ t('usage.imageBillingSize') }}</span>
                <span class="font-medium text-white">{{ formatImageBillingSize(tooltipData, t) }}</span>
              </div>
              <div class="flex items-center justify-between gap-4">
                <span class="text-gray-400">{{ t('usage.imageSizeSource') }}</span>
                <span class="font-medium text-white">{{ formatImageSizeSource(tooltipData, t) }}</span>
              </div>
              <div class="flex items-center justify-between gap-4">
                <span class="text-gray-400">{{ t('usage.imageInputSize') }}</span>
                <span class="font-medium text-white">{{ formatImageInputSize(tooltipData, t) }}</span>
              </div>
              <div class="flex items-center justify-between gap-4">
                <span class="text-gray-400">{{ t('usage.imageOutputSize') }}</span>
                <span class="font-medium text-white">{{ formatImageOutputSize(tooltipData, t) }}</span>
              </div>
              <div v-if="formatImageSizeBreakdown(tooltipData)" class="flex items-center justify-between gap-4">
                <span class="text-gray-400">{{ t('usage.imageSizeBreakdown') }}</span>
                <span class="font-medium text-white">{{ formatImageSizeBreakdown(tooltipData) }}</span>
              </div>
              <div class="flex items-center justify-between gap-4">
                <span class="text-gray-400">{{ t('usage.imageUnitPrice') }}</span>
                <span class="font-medium text-sky-300">${{ imageUnitPrice(tooltipData).toFixed(6) }}</span>
              </div>
              <div class="flex items-center justify-between gap-4">
                <span class="text-gray-400">{{ t('usage.imageTotalPrice') }}</span>
                <span class="font-medium text-white">${{ tooltipData.total_cost?.toFixed(6) || '0.000000' }}</span>
              </div>
            </template>
            <div v-else class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('usage.unitPrice') }}</span>
              <span class="font-medium text-sky-300">${{ tooltipData?.total_cost?.toFixed(6) || '0.000000' }}</span>
            </div>
            <div v-if="tooltipData && tooltipData.cache_creation_cost > 0" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('admin.usage.cacheCreationCost') }}</span>
              <span class="font-medium text-white">${{ tooltipData.cache_creation_cost.toFixed(6) }}</span>
            </div>
            <div v-if="tooltipData && tooltipData.cache_read_cost > 0" class="flex items-center justify-between gap-4">
              <span class="text-gray-400">{{ t('admin.usage.cacheReadCost') }}</span>
              <span class="font-medium text-white">${{ tooltipData.cache_read_cost.toFixed(6) }}</span>
            </div>
          </div>
          <!-- Rate and Summary -->
          <div class="flex items-center justify-between gap-6">
            <span class="text-gray-400">{{ t('usage.serviceTier') }}</span>
            <span class="font-semibold text-cyan-300">{{ getUsageServiceTierLabel(tooltipData?.service_tier, t) }}</span>
          </div>
          <div class="flex items-center justify-between gap-6">
            <span class="text-gray-400">{{ t('usage.rate') }}</span>
            <span class="font-semibold text-blue-400"
              >{{ formatMultiplier(tooltipData?.rate_multiplier || 1) }}x</span
            >
          </div>
          <div class="flex items-center justify-between gap-6">
            <span class="text-gray-400">{{ t('usage.original') }}</span>
            <span class="font-medium text-white">${{ tooltipData?.total_cost.toFixed(6) }}</span>
          </div>
          <div class="flex items-center justify-between gap-6 border-t border-gray-700 pt-1.5">
            <span class="text-gray-400">{{ t('usage.billed') }}</span>
            <span class="font-semibold text-green-400"
              >${{ tooltipData?.actual_cost.toFixed(6) }}</span
            >
          </div>
        </div>
        <!-- Tooltip Arrow (left side) -->
        <div
          class="absolute right-full top-1/2 h-0 w-0 -translate-y-1/2 border-b-[6px] border-r-[6px] border-t-[6px] border-b-transparent border-r-gray-900 border-t-transparent dark:border-r-gray-800"
        ></div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, reactive, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { usageAPI, keysAPI } from '@/api'
import { adminUsageAPI } from '@/api/admin/usage'
import type {
  AdminUsageCalibration,
  AdminUsageCalibrationMode,
  CreateAdminUsageCalibrationRequest,
  SimpleApiKey,
  SimpleUser
} from '@/api/admin/usage'
import { usersAPI } from '@/api/admin/users'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select from '@/components/common/Select.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import AdminUserSearchPicker from '@/components/admin/usage/AdminUserSearchPicker.vue'
import UserErrorRequestsTable from '@/components/user/UserErrorRequestsTable.vue'
import type { UsageLog, ApiKey, UsageQueryParams, UsageStatsResponse, UserErrorRequest, AdminUser } from '@/types'
import type { Column } from '@/components/common/types'
import { formatDateTime, formatReasoningEffort } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatCacheTokens, formatMultiplier } from '@/utils/formatters'
import { formatTokenPricePerMillion } from '@/utils/usagePricing'
import { getUsageServiceTierLabel } from '@/utils/usageServiceTier'
import { resolveUsageRequestType } from '@/utils/usageRequestType'
import {
  LATENCY_BAR_CLASSES,
  LATENCY_BAR_FROM_CLASSES,
  LATENCY_BAR_TO_CLASSES,
  LATENCY_TEXT_CLASSES,
  durationSeverity,
  firstTokenSeverity,
} from '@/utils/latencyHealth'
import {
  BILLING_MODE_TOKEN,
  getBillingModeBadgeClass,
  getBillingModeLabel,
  isImageUsage,
  getDisplayBillingMode,
  imageUnitPrice,
} from '@/utils/billingMode'
import {
  formatImageBillingSize,
  formatImageInputSize,
  formatImageOutputSize,
  formatImageSizeBreakdown,
  formatImageSizeSource,
  hasImageOutputTokens,
  textOutputTokens,
  hasImageOutputCost,
} from '@/utils/imageUsage'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const isAdmin = computed(() => authStore.isAdmin)

let abortController: AbortController | null = null

// Tooltip state
const tooltipVisible = ref(false)
const tooltipPosition = ref({ x: 0, y: 0 })
const tooltipData = ref<UsageLog | null>(null)

// Token tooltip state
const tokenTooltipVisible = ref(false)
const tokenTooltipPosition = ref({ x: 0, y: 0 })
const tokenTooltipData = ref<UsageLog | null>(null)

// Usage stats from API
const usageStats = ref<UsageStatsResponse | null>(null)

type CalibrationFormState = {
  tokenEnabled: boolean
  tokenMode: AdminUsageCalibrationMode
  tokenValue: string
  tokenStartDate: string
  tokenEndDate: string
  balanceEnabled: boolean
  balanceMode: AdminUsageCalibrationMode
  balanceValue: string
  consumptionEnabled: boolean
  consumptionMode: AdminUsageCalibrationMode
  consumptionValue: string
}

const adminUsers = ref<SimpleUser[]>([])
const adminSelectedUserValue = ref<number | null>(null)
const selectedAdminUser = ref<SimpleUser | null>(null)
const selectedAdminUserDetail = ref<AdminUser | null>(null)
const loadingAdminUsers = ref(false)

const calibrationDialogVisible = ref(false)
const submittingCalibration = ref(false)
const loadingCalibrations = ref(false)
const revokingCalibrationID = ref<number | null>(null)
const calibrationHistory = ref<AdminUsageCalibration[]>([])
const calibrationTokenCurrentTotal = ref(0)
const calibrationConsumptionCurrentTotal = ref(0)
const calibrationForm = reactive<CalibrationFormState>({
  tokenEnabled: true,
  tokenMode: 'delta',
  tokenValue: '',
  tokenStartDate: '',
  tokenEndDate: '',
  balanceEnabled: false,
  balanceMode: 'delta',
  balanceValue: '',
  consumptionEnabled: false,
  consumptionMode: 'delta',
  consumptionValue: ''
})

const selectedAdminUserID = computed(() => {
  return typeof adminSelectedUserValue.value === 'number' ? adminSelectedUserValue.value : null
})

const isAdminUserViewActive = computed(() => isAdmin.value && !!selectedAdminUserID.value)

const adminUserIsDeleted = (user: SimpleUser | AdminUser | null): boolean => {
  if (!user) return false
  if ('deleted' in user && user.deleted) return true
  return 'deleted_at' in user && !!user.deleted_at
}

const selectedAdminUserDeleted = computed(() => {
  return adminUserIsDeleted(selectedAdminUserDetail.value) || adminUserIsDeleted(selectedAdminUser.value)
})

const canCalibrateSelectedAdminUser = computed(() => {
  return !!selectedAdminUserID.value && !selectedAdminUserDeleted.value
})

const calibrationButtonTitle = computed(() => {
  if (!selectedAdminUserID.value) return t('usage.adminSelectUserFirst')
  if (selectedAdminUserDeleted.value) return t('usage.adminDeletedUserCannotCalibrate')
  return t('usage.adminCalibration')
})

// 缓存命中率 = cache_read / (input + cache_read)
// 分母为 0（无任何输入）时显示 '-'
const cacheStats = computed(() => {
  // 总输入 token = 普通输入 + 缓存写入 + 缓存读取（命中）
  // 缓存命中率 = 缓存读取 / 总输入；总输入为 0 时返回零值，模板按 '-' 渲染。
  const cacheRead = usageStats.value?.total_cache_read_tokens || 0
  const cacheCreate = usageStats.value?.total_cache_creation_tokens || 0
  const input = usageStats.value?.total_input_tokens || 0
  const totalInput = input + cacheCreate + cacheRead
  const ratePercent = totalInput > 0 ? `${((cacheRead / totalInput) * 100).toFixed(1)}%` : '-'
  return { cacheRead, totalInput, ratePercent }
})

const columns = computed<Column[]>(() => [
  { key: 'api_key', label: t('usage.apiKeyFilter'), sortable: false },
  { key: 'model', label: t('usage.model'), sortable: true },
  { key: 'reasoning_effort', label: t('usage.reasoningEffort'), sortable: false },
  { key: 'endpoint', label: t('usage.endpoint'), sortable: false },
  { key: 'stream', label: t('usage.type'), sortable: false },
  { key: 'billing_mode', label: t('admin.usage.billingMode'), sortable: false },
  { key: 'tokens', label: t('usage.tokens'), sortable: false },
  { key: 'cost', label: t('usage.cost'), sortable: false },
  { key: 'latency', label: t('usage.latency'), sortable: false },
  { key: 'created_at', label: t('usage.time'), sortable: true },
  { key: 'user_agent', label: t('usage.userAgent'), sortable: false }
])

const usageLogs = ref<UsageLog[]>([])
const apiKeys = ref<ApiKey[]>([])
const loading = ref(false)

const apiKeyOptions = computed(() => {
  return [
    { value: null, label: t('usage.allApiKeys') },
    ...apiKeys.value.map((key) => ({
      value: key.id,
      label: key.name
    }))
  ]
})

const calibrationModeOptions = computed(() => [
  { value: 'delta', label: t('usage.adminCalibrationModeDelta') },
  { value: 'target', label: t('usage.adminCalibrationModeTarget') }
])

const toSimpleAdminUser = (user: SimpleUser | AdminUser | null): SimpleUser | null => {
  if (!user) return null
  return {
    id: user.id,
    email: user.email || `#${user.id}`,
    deleted: adminUserIsDeleted(user)
  }
}

const selectedAdminSearchUser = computed(() => {
  if (selectedAdminUserDetail.value?.id === adminSelectedUserValue.value) {
    return toSimpleAdminUser(selectedAdminUserDetail.value)
  }
  return selectedAdminUser.value
})

const formatAdminUserOption = (user: SimpleUser | AdminUser): string => {
  const email = user.email || `#${user.id}`
  return adminUserIsDeleted(user) ? `${email} (${t('usage.adminDeletedUser')})` : email
}

const selectedAdminUserLabel = computed(() => {
  if (selectedAdminUserDetail.value) {
    return formatAdminUserOption(selectedAdminUserDetail.value)
  }
  if (selectedAdminUser.value) {
    return formatAdminUserOption(selectedAdminUser.value)
  }
  return t('usage.adminSelectUserPlaceholder')
})

// Helper function to format date in local timezone
const formatLocalDate = (date: Date): string => {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

// Initialize date range immediately
const now = new Date()
const weekAgo = new Date(now)
weekAgo.setDate(weekAgo.getDate() - 6)

// Date range state
const startDate = ref(formatLocalDate(weekAgo))
const endDate = ref(formatLocalDate(now))

const filters = ref<UsageQueryParams>({
  api_key_id: undefined,
  start_date: undefined,
  end_date: undefined
})

// Initialize filters with date range
filters.value.start_date = startDate.value
filters.value.end_date = endDate.value

// Handle date range change from DateRangePicker
const onDateRangeChange = (range: {
  startDate: string
  endDate: string
  preset: string | null
}) => {
  filters.value.start_date = range.startDate
  filters.value.end_date = range.endDate
  applyFilters()
  errorPage.value = 1
  if (activeTab.value === 'errors') {
    loadErrors()
  } else {
    errorRows.value = []  // 失效，下次切到 errors tab 时按新日期重新加载
  }
}

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})
const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

const formatDuration = (ms: number | null | undefined): string => {
  if (ms == null) return '-'
  if (ms < 1000) return `${ms}ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(2)}s`
  const totalSec = Math.round(ms / 1000)
  if (totalSec < 3600) return `${Math.floor(totalSec / 60)}m ${totalSec % 60}s`
  return `${Math.floor(totalSec / 3600)}h ${Math.floor((totalSec % 3600) / 60)}m`
}


const formatUserAgent = (ua: string): string => {
  return ua
}

const getRequestTypeLabel = (log: UsageLog): string => {
  const requestType = resolveUsageRequestType(log)
  if (requestType === 'cyber') return t('usage.cyber')
  if (requestType === 'ws_v2') return t('usage.ws')
  if (requestType === 'stream') return t('usage.stream')
  if (requestType === 'sync') return t('usage.sync')
  return t('usage.unknown')
}

const getRequestTypeBadgeClass = (log: UsageLog): string => {
  const requestType = resolveUsageRequestType(log)
  if (requestType === 'cyber') return 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
  if (requestType === 'ws_v2') return 'bg-violet-100 text-violet-800 dark:bg-violet-900 dark:text-violet-200'
  if (requestType === 'stream') return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'
  if (requestType === 'sync') return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200'
  return 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200'
}


const formatUsageEndpoints = (log: UsageLog): string => {
  const inbound = log.inbound_endpoint?.trim()
  return inbound || '-'
}

const formatTokens = (value: number): string => {
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
  return value.toLocaleString()
}

const formatSignedTokens = (value: number): string => {
  const sign = value > 0 ? '+' : value < 0 ? '-' : ''
  return `${sign}${formatTokens(Math.abs(value))}`
}

const parseIntegerInput = (value: string | number | null | undefined): number | null => {
  const trimmed = String(value ?? '').trim()
  if (!trimmed) return null
  const parsed = Number(trimmed)
  if (!Number.isFinite(parsed)) return null
  return Math.trunc(parsed)
}

const parseNumberInput = (value: string | number | null | undefined): number | null => {
  const trimmed = String(value ?? '').trim()
  if (!trimmed) return null
  const parsed = Number(trimmed)
  return Number.isFinite(parsed) ? parsed : null
}

const formatSignedInteger = (value: number): string => {
  const sign = value > 0 ? '+' : ''
  return `${sign}${Math.trunc(value).toLocaleString()}`
}

const formatSignedMoney = (value: number): string => {
  const sign = value > 0 ? '+' : ''
  return `${sign}$${value.toFixed(6)}`
}

const browserTimezone = (): string => {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
}

type UsageTableQueryParams = UsageQueryParams & {
  sort_by?: string
  sort_order?: 'asc' | 'desc'
}

const buildUsageQueryParams = (page: number, pageSize: number): UsageTableQueryParams => {
  const params: UsageTableQueryParams = {
    page,
    page_size: pageSize,
    ...filters.value,
    sort_by: sortState.sort_by,
    sort_order: sortState.sort_order
  }
  if (params.api_key_id == null) {
    delete params.api_key_id
  }
  return params
}

const fetchUsageLogsPage = async (
  page: number,
  pageSize: number,
  options: { signal?: AbortSignal } = {}
) => {
  const params = buildUsageQueryParams(page, pageSize)
  if (isAdminUserViewActive.value && selectedAdminUserID.value) {
    return adminUsageAPI.getUserView(
      {
        ...params,
        user_id: selectedAdminUserID.value
      },
      options
    )
  }
  return usageAPI.query(params, options)
}

const toApiKeyLike = (key: SimpleApiKey): ApiKey => ({
  id: key.id,
  user_id: key.user_id,
  key: '',
  name: key.name,
  group_id: null,
  status: 'active',
  ip_whitelist: [],
  ip_blacklist: [],
  last_used_at: null,
  last_used_ip: null,
  quota: 0,
  quota_used: 0,
  current_concurrency: 0,
  expires_at: null,
  created_at: '',
  updated_at: '',
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0,
  usage_5h: 0,
  usage_1d: 0,
  usage_7d: 0,
  window_5h_start: null,
  window_1d_start: null,
  window_7d_start: null,
  reset_5h_at: null,
  reset_1d_at: null,
  reset_7d_at: null
})

const loadUsageLogs = async () => {
  if (abortController) {
    abortController.abort()
  }
  const currentAbortController = new AbortController()
  abortController = currentAbortController
  const { signal } = currentAbortController
  loading.value = true
  try {
    const response = await fetchUsageLogsPage(pagination.page, pagination.page_size, { signal })
    if (signal.aborted) {
      return
    }
    usageLogs.value = response.items
    pagination.total = response.total
    pagination.pages = response.pages
  } catch (error) {
    if (signal.aborted) {
      return
    }
    const abortError = error as { name?: string; code?: string }
    if (abortError?.name === 'AbortError' || abortError?.code === 'ERR_CANCELED') {
      return
    }
    appStore.showError(t('usage.failedToLoad'))
  } finally {
    if (abortController === currentAbortController) {
      loading.value = false
    }
  }
}

const loadApiKeys = async () => {
  try {
    if (isAdminUserViewActive.value && selectedAdminUserID.value) {
      const keys = await adminUsageAPI.searchApiKeys(selectedAdminUserID.value)
      apiKeys.value = keys.map(toApiKeyLike)
    } else {
      const response = await keysAPI.list(1, 100)
      apiKeys.value = response.items
    }
  } catch (error) {
    console.error('Failed to load API keys:', error)
  }
}

const loadUsageStats = async () => {
  try {
    const apiKeyId = filters.value.api_key_id ? Number(filters.value.api_key_id) : undefined
    const rangeStart = filters.value.start_date || startDate.value
    const rangeEnd = filters.value.end_date || endDate.value
    const stats = isAdminUserViewActive.value && selectedAdminUserID.value
      ? await adminUsageAPI.getUserViewStats({
          user_id: selectedAdminUserID.value,
          start_date: rangeStart,
          end_date: rangeEnd,
          api_key_id: apiKeyId
        })
      : await usageAPI.getStatsByDateRange(rangeStart, rangeEnd, apiKeyId)
    usageStats.value = stats
  } catch (error) {
    console.error('Failed to load usage stats:', error)
  }
}

const searchAdminUsers = async (keyword: string) => {
  const trimmed = keyword.trim()
  if (!trimmed) {
    adminUsers.value = []
    return
  }
  loadingAdminUsers.value = true
  adminUsers.value = []
  try {
    adminUsers.value = await adminUsageAPI.searchUsers(trimmed)
  } catch (error) {
    console.error('Failed to search users:', error)
    appStore.showError(t('usage.adminSearchUsersFailed'))
  } finally {
    loadingAdminUsers.value = false
  }
}

const selectAdminUserByID = async (value: number) => {
  const found = adminUsers.value.find((user) => user.id === value)
  selectedAdminUser.value = found ?? { id: value, email: `#${value}`, deleted: false }
  adminSelectedUserValue.value = value
  filters.value.api_key_id = undefined
  pagination.page = 1
  activeTab.value = 'usage'
  await loadSelectedAdminUserDetail(value)
  await loadApiKeys()
  calibrationHistory.value = []
  applyFilters()
}

const loadSelectedAdminUserDetail = async (userID: number) => {
  try {
    selectedAdminUserDetail.value = await usersAPI.getById(userID, true)
  } catch (error) {
    console.error('Failed to load selected user:', error)
    selectedAdminUserDetail.value = null
    appStore.showError(t('usage.adminLoadUserFailed'))
  }
}

const clearAdminUserSelection = async () => {
  if (!isAdmin.value) {
    return
  }
  adminSelectedUserValue.value = null
  selectedAdminUser.value = null
  selectedAdminUserDetail.value = null
  adminUsers.value = []
  calibrationHistory.value = []
  filters.value.api_key_id = undefined
  pagination.page = 1
  activeTab.value = 'usage'
  await loadApiKeys()
  applyFilters()
}

const handleAdminUserSelect = async (user: SimpleUser) => {
  if (!isAdmin.value) {
    return
  }
  await selectAdminUserByID(user.id)
}

const tokenValueLabel = computed(() => {
  return calibrationForm.tokenMode === 'target'
    ? t('usage.adminTokenTargetValue')
    : t('usage.adminTokenDeltaValue')
})

const balanceValueLabel = computed(() => {
  return calibrationForm.balanceMode === 'target'
    ? t('usage.adminBalanceTargetValue')
    : t('usage.adminBalanceDeltaValue')
})

const consumptionValueLabel = computed(() => {
  return calibrationForm.consumptionMode === 'target'
    ? t('usage.adminConsumptionTargetValue')
    : t('usage.adminConsumptionDeltaValue')
})

const calibrationCurrentBalance = computed(() => selectedAdminUserDetail.value?.balance ?? 0)

const calibrationTokenDelta = computed(() => {
  if (!calibrationForm.tokenEnabled) return 0
  const value = parseIntegerInput(calibrationForm.tokenValue)
  if (value == null) return 0
  if (calibrationForm.tokenMode === 'target') {
    return value - calibrationTokenCurrentTotal.value
  }
  return value
})

const calibrationBalanceDelta = computed(() => {
  if (!calibrationForm.balanceEnabled) return 0
  const value = parseNumberInput(calibrationForm.balanceValue)
  if (value == null) return 0
  if (calibrationForm.balanceMode === 'target') {
    return value - calibrationCurrentBalance.value
  }
  return value
})

const calibrationConsumptionDelta = computed(() => {
  if (!calibrationForm.consumptionEnabled) return 0
  const value = parseNumberInput(calibrationForm.consumptionValue)
  if (value == null) return 0
  if (calibrationForm.consumptionMode === 'target') {
    return value - calibrationConsumptionCurrentTotal.value
  }
  return value
})

let calibrationStatsRequestSeq = 0
const loadCalibrationTokenCurrentTotal = async () => {
  if (
    !calibrationDialogVisible.value ||
    (!calibrationForm.tokenEnabled && !calibrationForm.consumptionEnabled) ||
    !selectedAdminUserID.value ||
    !calibrationForm.tokenStartDate ||
    !calibrationForm.tokenEndDate
  ) {
    calibrationTokenCurrentTotal.value = 0
    calibrationConsumptionCurrentTotal.value = 0
    return
  }
  const seq = ++calibrationStatsRequestSeq
  try {
    const stats = await adminUsageAPI.getUserViewStats({
      user_id: selectedAdminUserID.value,
      start_date: calibrationForm.tokenStartDate,
      end_date: calibrationForm.tokenEndDate,
      timezone: browserTimezone()
    })
    if (seq === calibrationStatsRequestSeq) {
      calibrationTokenCurrentTotal.value = stats.total_tokens || 0
      calibrationConsumptionCurrentTotal.value = stats.total_actual_cost || 0
    }
  } catch (error) {
    if (seq === calibrationStatsRequestSeq) {
      calibrationTokenCurrentTotal.value = 0
      calibrationConsumptionCurrentTotal.value = 0
    }
    console.error('Failed to load calibration token preview:', error)
  }
}

const loadCalibrationHistory = async () => {
  if (!selectedAdminUserID.value) {
    calibrationHistory.value = []
    return
  }
  loadingCalibrations.value = true
  try {
    const response = await adminUsageAPI.listCalibrations({
      user_id: selectedAdminUserID.value,
      page: 1,
      page_size: 5
    })
    calibrationHistory.value = response.items
  } catch (error) {
    console.error('Failed to load calibration history:', error)
    appStore.showError(t('usage.adminLoadCalibrationHistoryFailed'))
  } finally {
    loadingCalibrations.value = false
  }
}

const resetCalibrationForm = () => {
  calibrationForm.tokenEnabled = true
  calibrationForm.tokenMode = 'delta'
  calibrationForm.tokenValue = ''
  calibrationForm.tokenStartDate = filters.value.start_date || startDate.value
  calibrationForm.tokenEndDate = filters.value.end_date || endDate.value
  calibrationForm.balanceEnabled = false
  calibrationForm.balanceMode = 'delta'
  calibrationForm.balanceValue = ''
  calibrationForm.consumptionEnabled = false
  calibrationForm.consumptionMode = 'delta'
  calibrationForm.consumptionValue = ''
  calibrationTokenCurrentTotal.value = usageStats.value?.total_tokens || 0
  calibrationConsumptionCurrentTotal.value = usageStats.value?.total_actual_cost || 0
}

const openCalibrationDialog = async () => {
  if (!selectedAdminUserID.value) {
    appStore.showWarning(t('usage.adminSelectUserFirst'))
    return
  }
  if (selectedAdminUserDeleted.value) {
    appStore.showWarning(t('usage.adminDeletedUserCannotCalibrate'))
    return
  }
  resetCalibrationForm()
  calibrationDialogVisible.value = true
  await Promise.all([
    loadSelectedAdminUserDetail(selectedAdminUserID.value),
    loadCalibrationHistory(),
    loadCalibrationTokenCurrentTotal()
  ])
}

const closeCalibrationDialog = () => {
  if (submittingCalibration.value) {
    return
  }
  calibrationDialogVisible.value = false
}

const validateCalibrationForm = (): CreateAdminUsageCalibrationRequest | null => {
  const targetUserID = selectedAdminUserID.value
  if (!targetUserID) {
    appStore.showWarning(t('usage.adminSelectUserFirst'))
    return null
  }
  if (selectedAdminUserDeleted.value) {
    appStore.showWarning(t('usage.adminDeletedUserCannotCalibrate'))
    return null
  }
  if (!calibrationForm.tokenEnabled && !calibrationForm.balanceEnabled && !calibrationForm.consumptionEnabled) {
    appStore.showWarning(t('usage.adminCalibrationSelectAtLeastOne'))
    return null
  }
  if (calibrationForm.balanceEnabled && calibrationForm.consumptionEnabled) {
    appStore.showWarning(t('usage.adminBalanceConsumptionExclusive'))
    return null
  }

  const payload: CreateAdminUsageCalibrationRequest = {
    target_user_id: targetUserID
  }

  if (calibrationForm.tokenEnabled || calibrationForm.consumptionEnabled) {
    if (!calibrationForm.tokenStartDate || !calibrationForm.tokenEndDate) {
      appStore.showWarning(t('usage.adminTokenRangeRequired'))
      return null
    }
    if (calibrationForm.tokenStartDate > calibrationForm.tokenEndDate) {
      appStore.showWarning(t('usage.adminTokenRangeInvalid'))
      return null
    }
  }

  if (calibrationForm.tokenEnabled) {
    const tokenValue = parseIntegerInput(calibrationForm.tokenValue)
    if (tokenValue == null) {
      appStore.showWarning(t('usage.adminTokenValueRequired'))
      return null
    }
    if (calibrationForm.tokenMode === 'target' && tokenValue < 0) {
      appStore.showWarning(t('usage.adminTokenTargetInvalid'))
      return null
    }
    payload.token = {
      mode: calibrationForm.tokenMode,
      value: tokenValue,
      start_date: calibrationForm.tokenStartDate,
      end_date: calibrationForm.tokenEndDate,
      timezone: browserTimezone()
    }
  }

  if (calibrationForm.balanceEnabled) {
    const balanceValue = parseNumberInput(calibrationForm.balanceValue)
    if (balanceValue == null) {
      appStore.showWarning(t('usage.adminBalanceValueRequired'))
      return null
    }
    if (calibrationForm.balanceMode === 'target' && balanceValue < 0) {
      appStore.showWarning(t('usage.adminBalanceTargetInvalid'))
      return null
    }
    if (calibrationCurrentBalance.value + calibrationBalanceDelta.value < 0) {
      appStore.showWarning(t('usage.adminBalanceNegativeInvalid'))
      return null
    }
    payload.balance = {
      mode: calibrationForm.balanceMode,
      value: balanceValue
    }
  }

  if (calibrationForm.consumptionEnabled) {
    const consumptionValue = parseNumberInput(calibrationForm.consumptionValue)
    if (consumptionValue == null) {
      appStore.showWarning(t('usage.adminConsumptionValueRequired'))
      return null
    }
    if (calibrationForm.consumptionMode === 'target' && consumptionValue < 0) {
      appStore.showWarning(t('usage.adminConsumptionTargetInvalid'))
      return null
    }
    if (calibrationConsumptionCurrentTotal.value + calibrationConsumptionDelta.value < 0) {
      appStore.showWarning(t('usage.adminConsumptionNegativeInvalid'))
      return null
    }
    if (calibrationCurrentBalance.value - calibrationConsumptionDelta.value < 0) {
      appStore.showWarning(t('usage.adminConsumptionBalanceInsufficient'))
      return null
    }
    payload.consumption = {
      mode: calibrationForm.consumptionMode,
      value: consumptionValue,
      start_date: calibrationForm.tokenStartDate,
      end_date: calibrationForm.tokenEndDate,
      timezone: browserTimezone()
    }
  }

  return payload
}

const submitCalibration = async () => {
  const payload = validateCalibrationForm()
  if (!payload) {
    return
  }
  submittingCalibration.value = true
  try {
    const idempotencyKey = `admin-usage-calibration-${payload.target_user_id}-${Date.now()}-${Math.random().toString(36).slice(2)}`
    await adminUsageAPI.createCalibration(payload, idempotencyKey)
    appStore.showSuccess(t('usage.adminCalibrationSuccess'))
    calibrationDialogVisible.value = false
    await Promise.all([
      loadUsageStats(),
      loadSelectedAdminUserDetail(payload.target_user_id),
      loadCalibrationHistory()
    ])
  } catch (error) {
    console.error('Failed to submit calibration:', error)
    appStore.showError(extractApiErrorMessage(error, t('usage.adminCalibrationFailed')))
  } finally {
    submittingCalibration.value = false
  }
}

const revokeCalibration = async (item: AdminUsageCalibration) => {
  if (item.revoked_at || revokingCalibrationID.value !== null) return
  if (!window.confirm(t('usage.adminCalibrationRevokeConfirm'))) return
  revokingCalibrationID.value = item.id
  try {
    await adminUsageAPI.revokeCalibration(item.id)
    appStore.showSuccess(t('usage.adminCalibrationRevokeSuccess'))
    await Promise.all([
      loadCalibrationHistory(),
      loadUsageStats(),
      loadSelectedAdminUserDetail(item.target_user_id)
    ])
  } catch (error) {
    console.error('Failed to revoke calibration:', error)
    appStore.showError(extractApiErrorMessage(error, t('usage.adminCalibrationRevokeFailed')))
  } finally {
    revokingCalibrationID.value = null
  }
}

const formatCalibrationSummary = (item: AdminUsageCalibration): string => {
  const parts: string[] = []
  if (typeof item.token_delta === 'number') {
    parts.push(`${t('usage.adminTokenCalibration')}: ${formatSignedInteger(item.token_delta)}`)
  }
  if (typeof item.consumption_delta === 'number') {
    parts.push(`${t('usage.adminConsumptionCalibration')}: ${formatSignedMoney(item.consumption_delta)}`)
  } else if (typeof item.balance_delta === 'number') {
    parts.push(`${t('usage.adminBalanceCalibration')}: ${formatSignedMoney(item.balance_delta)}`)
  }
  return parts.join(' · ') || t('usage.adminCalibration')
}

watch(
  () => [
    calibrationDialogVisible.value,
    calibrationForm.tokenEnabled,
    calibrationForm.consumptionEnabled,
    calibrationForm.tokenStartDate,
    calibrationForm.tokenEndDate,
    selectedAdminUserID.value
  ],
  () => {
    loadCalibrationTokenCurrentTotal()
  }
)

const applyFilters = () => {
  pagination.page = 1
  loadUsageLogs()
  loadUsageStats()
}

const resetFilters = () => {
  filters.value = {
    api_key_id: undefined,
    start_date: undefined,
    end_date: undefined
  }
  // Reset date range to default (last 7 days)
  const now = new Date()
  const weekAgo = new Date(now)
  weekAgo.setDate(weekAgo.getDate() - 6)
  startDate.value = formatLocalDate(weekAgo)
  endDate.value = formatLocalDate(now)
  filters.value.start_date = startDate.value
  filters.value.end_date = endDate.value
  pagination.page = 1
  loadUsageLogs()
  loadUsageStats()
}

const handlePageChange = (page: number) => {
  pagination.page = page
  loadUsageLogs()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadUsageLogs()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadUsageLogs()
}

// Tooltip functions
const showTooltip = (event: MouseEvent, row: UsageLog) => {
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()

  tooltipData.value = row
  // Position to the right of the icon, vertically centered
  tooltipPosition.value.x = rect.right + 8
  tooltipPosition.value.y = rect.top + rect.height / 2
  tooltipVisible.value = true
}

const hideTooltip = () => {
  tooltipVisible.value = false
  tooltipData.value = null
}

// Token tooltip functions
const showTokenTooltip = (event: MouseEvent, row: UsageLog) => {
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()

  tokenTooltipData.value = row
  tokenTooltipPosition.value.x = rect.right + 8
  tokenTooltipPosition.value.y = rect.top + rect.height / 2
  tokenTooltipVisible.value = true
}

const hideTokenTooltip = () => {
  tokenTooltipVisible.value = false
  tokenTooltipData.value = null
}

// ── Error Requests Tab ──────────────────────────────────────────────────────
const activeTab = ref<'usage' | 'errors'>('usage')
const errorViewEnabled = computed(() => {
  return !isAdminUserViewActive.value && (appStore.cachedPublicSettings?.allow_user_view_error_requests ?? false)
})

const errorRows = ref<UserErrorRequest[]>([])
const errorLoading = ref(false)
const errorPage = ref(1)
const errorPageSize = ref(20)
const errorTotal = ref(0)
const errorFilter = ref<{ model: string; category: string; api_key_id: number | null }>({ model: '', category: '', api_key_id: null })

const loadErrors = async () => {
  errorLoading.value = true
  try {
    const resp = await usageAPI.listMyErrorRequests({
      page: errorPage.value,
      page_size: errorPageSize.value,
      start_date: startDate.value,
      end_date: endDate.value,
      model: errorFilter.value.model || undefined,
      category: errorFilter.value.category || undefined,
      api_key_id: errorFilter.value.api_key_id ?? undefined,
    })
    errorRows.value = resp.items
    errorTotal.value = resp.total
  } catch (error) {
    console.error('[UsageView] loadErrors failed:', error)
    appStore.showError(t('usage.errors.failedToLoad'))
  } finally {
    errorLoading.value = false
  }
}

const onErrorFilter = (f: { model: string; category: string; api_key_id: number | null }) => {
  errorFilter.value = f
  errorPage.value = 1
  loadErrors()
}
const onErrorPage = (p: number) => { errorPage.value = p; loadErrors() }
const onErrorPageSize = (s: number) => { errorPageSize.value = s; errorPage.value = 1; loadErrors() }

const switchToErrors = () => {
  activeTab.value = 'errors'
  if (errorRows.value.length === 0) loadErrors()
}

onMounted(() => {
  loadApiKeys()
  loadUsageLogs()
  loadUsageStats()
})
</script>
