<template>
  <div class="space-y-6">
    <div v-if="loading" class="flex justify-center py-12">
      <LoadingSpinner />
    </div>

    <EmptyState
      v-else-if="!home?.campaign"
      class="card min-h-80"
      :title="t('lotteryCampaign.emptyTitle')"
      :description="t('lotteryCampaign.emptyDescription')"
    />

    <template v-else>
      <!-- Hero with live countdown -->
      <div class="relative overflow-hidden rounded-2xl bg-gradient-to-br from-slate-900 via-teal-900 to-teal-700 p-6 text-white shadow-lg">
        <div class="pointer-events-none absolute inset-0 bg-mesh-gradient opacity-60" />
        <div class="pointer-events-none absolute -right-16 -top-20 h-56 w-56 rounded-full bg-white/10 blur-3xl" />
        <div class="pointer-events-none absolute -bottom-24 -left-10 h-56 w-56 rounded-full bg-teal-300/20 blur-3xl" />
        <div class="relative flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <div class="inline-flex rounded-full bg-white/15 px-3 py-1 text-xs font-medium">
                {{ t(`lotteryCampaign.statuses.${home.campaign.status}`) }}
              </div>
              <div v-if="isImminent" class="inline-flex items-center gap-1 rounded-full bg-amber-300/25 px-3 py-1 text-xs font-medium text-amber-50 motion-safe:animate-pulse-slow">
                <Icon name="fire" size="sm" />
                {{ t('lotteryCampaign.drawImminent') }}
              </div>
            </div>
            <h1 class="mt-4 text-2xl font-semibold sm:text-3xl">{{ home.campaign.name }}</h1>
            <p class="mt-2 max-w-3xl text-sm text-white/80">{{ campaignDescription }}</p>
            <div class="mt-5 flex flex-wrap gap-2.5">
              <div v-if="prizeTiers.length" class="inline-flex items-center gap-1.5 rounded-full bg-white/15 px-3 py-1.5 text-sm font-medium backdrop-blur">
                <Icon name="gift" size="sm" class="text-amber-100" />
                <span class="tabular-nums">{{ formatCents(totalPoolCents) }}</span>
                <span class="text-white/70">{{ t('lotteryCampaign.totalPool') }}</span>
              </div>
              <div v-if="prizeTiers.length" class="inline-flex items-center gap-1.5 rounded-full bg-white/15 px-3 py-1.5 text-sm font-medium backdrop-blur">
                <Icon name="gift" size="sm" class="text-sky-100" />
                <span class="tabular-nums">{{ totalWinnerSlots }}</span>
                <span class="text-white/70">{{ t('lotteryCampaign.totalWinners') }}</span>
              </div>
              <button type="button" class="inline-flex items-center gap-1.5 rounded-full bg-white/15 px-3 py-1.5 text-sm font-medium backdrop-blur transition hover:bg-white/25" @click="openParticipants">
                <Icon name="users" size="sm" class="text-emerald-100" />
                <span class="tabular-nums">{{ participantCount }}</span>
                <span class="text-white/70">{{ t('lotteryCampaign.participants') }}</span>
              </button>
            </div>
          </div>
          <div class="rounded-2xl bg-white/15 p-4 backdrop-blur motion-safe:animate-glow sm:min-w-[260px]">
            <p class="text-center text-sm text-white/75">{{ drawTimeLabel }}</p>
            <div v-if="hasCountdown" class="mt-3 flex items-stretch justify-center gap-1.5">
              <template v-for="(segment, index) in countdownSegments" :key="segment.label">
                <div class="flex min-w-[3rem] flex-col items-center rounded-xl bg-white/15 px-2 py-2">
                  <span class="text-2xl font-semibold tabular-nums leading-none">{{ segment.value }}</span>
                  <span class="mt-1 text-[0.65rem] uppercase tracking-wide text-white/70">{{ segment.label }}</span>
                </div>
                <span v-if="index < countdownSegments.length - 1" class="self-center text-xl font-semibold text-white/50">:</span>
              </template>
            </div>
            <p v-else class="mt-3 text-center text-lg font-semibold leading-tight">{{ nextDrawText }}</p>
            <p v-if="hasCountdown" class="mt-3 text-center text-xs text-white/70">{{ nextDrawText }}</p>
          </div>
        </div>
      </div>

      <div
        v-if="drawStatus === 'completed'"
        data-testid="lottery-draw-completed"
        class="flex items-start gap-3 border-l-4 border-amber-500 bg-amber-50 px-4 py-3 text-amber-950 dark:bg-amber-900/20 dark:text-amber-100"
      >
        <Icon name="gift" size="md" class="mt-0.5 shrink-0 text-amber-600 dark:text-amber-300" />
        <div>
          <p class="font-semibold">{{ t('lotteryCampaign.drawCompletedTitle') }}</p>
          <p class="mt-1 text-sm text-amber-800 dark:text-amber-200">{{ t('lotteryCampaign.drawCompletedDescription', { count: wheelWinners.length }) }}</p>
        </div>
      </div>

      <!-- Metrics -->
      <div class="grid gap-4 sm:grid-cols-3">
        <div class="card p-5">
          <div class="flex items-center justify-between gap-3">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ usageMetricLabel }}</p>
            <Icon name="bolt" size="sm" class="text-primary-500" />
          </div>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatUsage(currentUsage) }}</p>
        </div>
        <div class="card p-5">
          <div class="flex items-center justify-between gap-3">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('lotteryCampaign.threshold') }}</p>
            <Icon name="chart" size="sm" class="text-primary-500" />
          </div>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatUsage(thresholdUsage) }}</p>
        </div>
        <div class="card p-5">
          <div class="flex items-center justify-between gap-3">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ entryMetricLabel }}</p>
            <Icon name="sparkles" size="sm" class="text-primary-500" />
          </div>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ entryCount }}</p>
        </div>
      </div>

      <section v-if="participants.length || drawStatus === 'completed'" class="card overflow-hidden" aria-labelledby="lottery-wheel-title">
        <header class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <div class="flex flex-wrap items-center gap-2"><span class="h-2.5 w-2.5 rounded-full" :class="drawStatusClass" /><h2 id="lottery-wheel-title" class="text-base font-semibold text-gray-900 dark:text-white">{{ t('lotteryCampaign.wheelTitle') }}</h2><span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="drawStatusBadgeClass">{{ t(`lotteryCampaign.wheelStatuses.${drawStatus}`) }}</span></div>
            <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">{{ drawStatus === 'completed' ? t('lotteryCampaign.wheelCompletedDescription') : t('lotteryCampaign.wheelDescription') }}</p>
          </div>
          <div class="flex items-center gap-5 text-sm">
            <div><span class="text-gray-500 dark:text-dark-400">{{ t('lotteryCampaign.participants') }}</span><strong class="ml-2 tabular-nums text-gray-900 dark:text-white">{{ participants.length }}</strong></div>
            <div><span class="text-gray-500 dark:text-dark-400">{{ t('lotteryCampaign.totalWeight') }}</span><strong class="ml-2 tabular-nums text-gray-900 dark:text-white">{{ totalParticipantWeight }}</strong></div>
          </div>
        </header>
        <div class="grid gap-0 lg:grid-cols-[minmax(360px,1.05fr)_minmax(280px,.95fr)]">
          <div class="flex min-h-[390px] flex-col items-center justify-center gap-5 bg-gray-50/70 p-6 dark:bg-dark-900/40">
            <div class="relative aspect-square w-full max-w-[350px]">
              <div class="user-wheel-pointer" aria-hidden="true" />
              <div class="user-wheel" :style="userWheelStyle" role="img" :aria-label="t('lotteryCampaign.wheelAriaLabel')">
                <span v-for="segment in wheelSegments" :key="segment.key" class="user-wheel-label" :class="{ 'is-current': segment.isCurrent, 'is-winner': segment.isWinner }" :style="segment.labelStyle">{{ segment.shortEmail }}</span>
                <span class="user-wheel-hub">
                  <template v-if="drawStatus === 'completed'">
                    <strong>{{ wheelWinners.length }}</strong><small>{{ t('lotteryCampaign.wheelWinnerCount') }}</small>
                  </template>
                  <template v-else>
                    <strong>{{ myParticipant?.entry_count ?? entryCount }}</strong><small>{{ t('lotteryCampaign.myWeight') }}</small>
                  </template>
                </span>
              </div>
            </div>
            <div v-if="wheelWinners.length" data-testid="lottery-wheel-winners" class="w-full max-w-[350px] border-t border-amber-200 pt-4 dark:border-amber-800/60">
              <p class="text-center text-xs font-semibold uppercase text-amber-700 dark:text-amber-300">{{ t('lotteryCampaign.wheelWinners') }}</p>
              <div class="mt-2 flex flex-wrap justify-center gap-2">
                <span v-for="(winner, index) in wheelWinners.slice(0, 5)" :key="`${winner.entry_date}-${winner.created_at}-${index}`" class="inline-flex items-center gap-1.5 rounded-md bg-amber-100 px-2.5 py-1.5 text-xs font-semibold text-amber-900 dark:bg-amber-900/40 dark:text-amber-100">
                  <Icon name="gift" size="xs" />
                  {{ winner.masked_email }} · {{ winner.prize_name || t('lotteryCampaign.prize') }}
                </span>
              </div>
            </div>
          </div>
          <div class="flex min-h-[390px] flex-col p-5">
            <template v-if="drawStatus !== 'completed'">
              <div v-if="myParticipant" class="rounded-lg bg-primary-50 p-3 dark:bg-primary-900/20">
                <div class="flex items-center justify-between gap-3"><span class="text-sm font-semibold text-primary-800 dark:text-primary-200">{{ t('lotteryCampaign.mySector') }}</span><span class="rounded-full bg-white px-2 py-0.5 text-xs font-semibold text-primary-700 shadow-sm dark:bg-dark-800 dark:text-primary-200">{{ t('lotteryCampaign.weightTimes', { count: myParticipant.entry_count }) }}</span></div>
                <p class="mt-1 text-sm text-primary-700 dark:text-primary-300">{{ myParticipant.masked_email }}</p>
                <div class="mt-3 grid grid-cols-2 gap-3 border-t border-primary-100 pt-3 text-xs dark:border-primary-800/50"><div><span class="block text-primary-600/70 dark:text-primary-300/70">{{ t('lotteryCampaign.myProbability') }}</span><strong class="mt-1 block text-sm text-primary-900 dark:text-primary-100">{{ myProbability }}%</strong></div><div><span class="block text-primary-600/70 dark:text-primary-300/70">{{ t('lotteryCampaign.lastUpdated') }}</span><strong class="mt-1 block text-sm text-primary-900 dark:text-primary-100">{{ formatRelativeTime(participantsUpdatedAt) }}</strong></div></div>
              </div>
              <div class="mt-4 flex items-center justify-between"><h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('lotteryCampaign.weightDistribution') }}</h3><button type="button" class="text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-300" @click="openParticipants">{{ t('lotteryCampaign.viewAll') }}</button></div>
              <ol class="mt-2 divide-y divide-gray-100 dark:divide-dark-700">
                <li v-for="participant in rankedParticipants.slice(0, 7)" :key="participant.key" class="flex items-center gap-3 py-2.5">
                  <span class="h-2.5 w-2.5 shrink-0 rounded-sm" :style="{ backgroundColor: participant.color }" />
                  <span class="min-w-0 flex-1 truncate text-sm" :class="participant.isCurrent ? 'font-semibold text-primary-700 dark:text-primary-200' : 'text-gray-700 dark:text-dark-200'">{{ participant.displayEmail }}<span v-if="participant.isCurrent"> {{ t('lotteryCampaign.me') }}</span></span>
                  <span class="text-xs tabular-nums text-gray-500 dark:text-dark-400">{{ participant.share }}%</span>
                  <span class="w-12 text-right text-sm font-semibold tabular-nums text-gray-900 dark:text-white">×{{ participant.entry_count }}</span>
                </li>
              </ol>
              <p class="mt-auto pt-4 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ t('lotteryCampaign.fairnessFormula', { probability: t('lotteryCampaign.probabilityFormula') }) }}</p>
            </template>
            <template v-else>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('lotteryCampaign.roundResults') }}</h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('lotteryCampaign.roundResultsDescription') }}</p>
              <p v-if="wheelWinners.length === 0" class="mt-5 text-sm text-gray-500 dark:text-dark-400">{{ t('lotteryCampaign.roundNoWinners') }}</p>
              <ol v-else class="mt-4 divide-y divide-gray-100 dark:divide-dark-700">
                <li v-for="(winner, index) in wheelWinners" :key="`${winner.entry_date}-${winner.created_at}-${index}`" class="flex items-center justify-between gap-3 py-3">
                  <div class="min-w-0"><p class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ winner.masked_email }}</p><p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ winner.prize_name || t('lotteryCampaign.prize') }}</p></div>
                  <span class="shrink-0 text-sm font-semibold text-emerald-600 dark:text-emerald-300">{{ formatCents(winner.reward_amount_cents) }}</span>
                </li>
              </ol>
            </template>
          </div>
        </div>
      </section>

      <!-- Prize wall -->
      <div v-if="prizeTiers.length" class="card p-5">
        <div class="flex items-center gap-2">
          <Icon name="gift" size="sm" class="text-primary-500" />
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('lotteryCampaign.prizePool') }}</h2>
        </div>

        <!-- Single prize: featured banner -->
        <div
          v-if="isSinglePrize"
          class="relative mt-3 overflow-hidden rounded-xl border border-amber-200 bg-gradient-to-br from-amber-50 via-white to-amber-50/40 p-4 shadow-sm motion-safe:animate-scale-in motion-safe:transition motion-safe:duration-200 hover:shadow-card-hover dark:border-amber-500/30 dark:from-amber-900/20 dark:via-dark-800 dark:to-dark-800"
        >
          <div class="pointer-events-none absolute -right-6 -top-6 text-amber-200/50 motion-safe:animate-pulse-slow dark:text-amber-500/10">
            <Icon name="gift" size="xl" class="h-24 w-24" />
          </div>
          <div class="relative flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div class="flex items-center gap-3">
              <span class="inline-flex h-11 w-11 items-center justify-center rounded-xl bg-gradient-to-br from-amber-300 to-amber-500 text-white shadow-sm dark:from-amber-400 dark:to-amber-600">
                <Icon name="gift" size="md" />
              </span>
              <div>
                <p class="text-base font-semibold text-amber-700 dark:text-amber-200">{{ prizeTiers[0].tier_name || t('lotteryCampaign.prize') }}</p>
                <p class="mt-0.5 flex items-center gap-1 text-xs text-gray-500 dark:text-dark-400">
                  <Icon name="users" size="xs" />
                  {{ t('lotteryCampaign.winnerCount', { count: prizeTiers[0].winner_count }) }}
                </p>
              </div>
            </div>
            <p class="text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatCents(prizeTiers[0].reward_amount_cents) }}</p>
          </div>
        </div>

        <!-- Multiple prizes: tier grid -->
        <div v-else class="mt-3 grid gap-2.5 sm:grid-cols-2 lg:grid-cols-3">
          <div
            v-for="(tier, index) in prizeTiers"
            :key="tier.id"
            class="group relative overflow-hidden rounded-lg border p-3 motion-safe:animate-scale-in motion-safe:transition motion-safe:duration-200 hover:-translate-y-0.5 hover:shadow-card-hover"
            :class="tierAccent(index).card"
          >
            <div class="flex items-center gap-2">
              <span class="inline-flex h-7 w-7 items-center justify-center rounded-full text-sm font-bold tabular-nums" :class="tierAccent(index).medal">
                {{ index + 1 }}
              </span>
              <p class="text-sm font-semibold" :class="tierAccent(index).label">{{ tier.tier_name || t('lotteryCampaign.prize') }}</p>
            </div>
            <p class="mt-2 text-base font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatCents(tier.reward_amount_cents) }}</p>
            <p class="mt-0.5 flex items-center gap-1 text-xs text-gray-500 dark:text-dark-400">
              <Icon name="users" size="xs" />
              {{ t('lotteryCampaign.winnerCount', { count: tier.winner_count }) }}
            </p>
          </div>
        </div>
      </div>
      <!-- Entry status + stepped ladder -->
      <div class="card p-5">
        <div class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ entryStatusTitle }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ entryStatusDescription }}</p>
          </div>
          <button
            v-if="home.campaign.participation_mode === 'manual'"
            class="btn btn-primary"
            type="button"
            :disabled="enrolling || myData?.entry_status === 'enrolled' || entryCount <= 0"
            @click="enroll"
          >
            <Icon name="sparkles" size="sm" />
            {{ enrolling ? t('common.processing') : t('lotteryCampaign.enroll') }}
          </button>
        </div>
        <div
          class="mt-4 h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700"
          role="progressbar"
          data-testid="lottery-threshold-progress"
          :aria-label="t('lotteryCampaign.thresholdProgress')"
          :aria-valuenow="progressPct"
          aria-valuemin="0"
          aria-valuemax="100"
        >
          <div class="h-full rounded-full bg-primary-500 transition-all" :style="{ width: `${progressPct}%` }" />
        </div>
        <div class="mt-2 flex items-center justify-between gap-2 text-sm">
          <p class="font-medium text-primary-600 dark:text-primary-300">{{ progressHintText }}</p>
          <p class="tabular-nums text-gray-500 dark:text-dark-400">{{ t('lotteryCampaign.thresholdProgressPercent', { percent: progressPct }) }}</p>
        </div>

        <!-- Stepped ladder -->
        <div v-if="showLadder" class="mt-4 border-t border-gray-100 pt-4 dark:border-dark-700">
          <div class="flex items-center justify-between">
            <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ entryLadderLabel }}</p>
            <p class="text-sm font-semibold text-primary-600 dark:text-primary-300 tabular-nums">{{ entryCount }} / {{ maxEntries }}</p>
          </div>
          <div class="mt-3 flex flex-wrap gap-1.5" :aria-label="entryLadderLabel">
            <span
              v-for="slot in maxEntries"
              :key="slot"
              class="h-2.5 flex-1 rounded-full transition-colors"
              :class="slot <= entryCount ? 'bg-primary-500' : 'bg-gray-100 dark:bg-dark-700'"
            />
          </div>
          <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">{{ ladderHintText }}</p>
        </div>
      </div>

      <!-- Rules -->
      <div v-if="rulesText" class="card p-5">
        <div class="flex items-center gap-2">
          <Icon name="document" size="sm" class="text-primary-500" />
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('lotteryCampaign.rulesTitle') }}</h2>
        </div>
        <p class="mt-3 whitespace-pre-line text-sm leading-relaxed text-gray-600 dark:text-dark-300">{{ rulesText }}</p>
      </div>

      <!-- Recent winners feed (social proof) -->
      <div class="card overflow-hidden">
        <div class="flex items-center gap-2 border-b border-gray-100 p-4 dark:border-dark-700">
          <Icon name="users" size="sm" class="text-primary-500" />
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('lotteryCampaign.recentWinners') }}</h2>
        </div>
        <div v-if="recentWinners.length === 0" class="p-6 text-sm text-gray-500 dark:text-dark-400">
          {{ t('lotteryCampaign.noRecentWinners') }}
        </div>
        <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
          <div
            v-for="(winner, index) in recentWinners"
            :key="`${winner.masked_email}-${winner.created_at}-${index}`"
            class="flex items-center justify-between gap-4 p-4"
          >
            <div class="flex min-w-0 items-center gap-3">
              <span class="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary-100 text-primary-600 dark:bg-primary-900/40 dark:text-primary-300">
                <Icon name="user" size="sm" />
              </span>
              <div class="min-w-0">
                <p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ winner.masked_email }}</p>
                <p class="mt-0.5 truncate text-xs text-gray-500 dark:text-dark-400">
                  {{ winner.prize_name || t('lotteryCampaign.prize') }} · {{ formatRelativeTime(winner.created_at) }}
                </p>
              </div>
            </div>
            <p class="shrink-0 text-sm font-semibold text-emerald-600 dark:text-emerald-300">{{ formatCents(winner.reward_amount_cents) }}</p>
          </div>
        </div>
      </div>

      <!-- My winners with ceremony -->
      <div class="card overflow-hidden">
        <div class="flex items-center gap-2 border-b border-gray-100 p-4 dark:border-dark-700">
          <Icon name="sparkles" size="sm" class="text-amber-500" />
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('lotteryCampaign.myWinners') }}</h2>
        </div>
        <div v-if="(myData?.winners.length ?? 0) === 0" class="p-6 text-sm text-gray-500 dark:text-dark-400">
          {{ t('lotteryCampaign.noWinners') }}
        </div>
        <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
          <div
            v-for="winner in myData?.winners"
            :key="winner.id"
            class="flex items-center justify-between gap-4 bg-gradient-to-r from-amber-50/60 to-transparent p-4 motion-safe:animate-scale-in dark:from-amber-900/10"
          >
            <div class="flex items-center gap-3">
              <span class="inline-flex h-10 w-10 items-center justify-center rounded-full bg-amber-100 text-amber-600 motion-safe:animate-glow dark:bg-amber-900/40 dark:text-amber-300">
                <Icon name="gift" size="sm" />
              </span>
              <div>
                <p class="font-medium text-gray-900 dark:text-white">{{ winner.prize_name || t('lotteryCampaign.prize') }}</p>
                <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(winner.created_at) }}</p>
              </div>
            </div>
            <p class="text-lg font-semibold text-emerald-600 dark:text-emerald-300">{{ formatCents(winner.reward_amount_cents) }}</p>
          </div>
        </div>
      </div>
    </template>
    <div v-if="participantsOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" @click.self="participantsOpen = false">
      <section class="card max-h-[min(32rem,calc(100vh-2rem))] w-full max-w-md overflow-hidden bg-white shadow-xl dark:bg-dark-800" role="dialog" aria-modal="true" :aria-label="t('lotteryCampaign.participantsTitle')">
        <div class="flex items-center justify-between border-b border-gray-100 p-4 dark:border-dark-700">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('lotteryCampaign.participantsTitle') }}</h2>
          <button type="button" class="btn btn-ghost btn-sm" :aria-label="t('common.close')" @click="participantsOpen = false">&times;</button>
        </div>
        <div class="max-h-[26rem] overflow-y-auto p-4">
          <div v-if="participantsLoading" class="flex justify-center py-8"><LoadingSpinner /></div>
          <p v-else-if="participants.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('lotteryCampaign.noParticipants') }}</p>
          <ul v-else class="divide-y divide-gray-100 dark:divide-dark-700">
            <li v-for="(participant, index) in participants" :key="`${participant.masked_email}-${index}`" class="flex items-center justify-between gap-4 py-3">
              <span class="min-w-0 truncate text-sm text-gray-700 dark:text-dark-200">{{ participant.masked_email }}</span>
              <span class="shrink-0 rounded-md bg-gray-100 px-2 py-1 text-xs font-semibold tabular-nums text-gray-700 dark:bg-dark-700 dark:text-dark-200">{{ t('lotteryCampaign.weightTimes', { count: participant.entry_count || 1 }) }}</span>
            </li>
          </ul>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import lotteryCampaignsAPI, { type LotteryCampaign, type LotteryMyData, type LotteryPrizeTier, type LotteryPublicWinner } from '@/api/lotteryCampaigns'
import { useAppStore, useAuthStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatTokenMillions } from '@/utils/usagePricing'

const { t, locale } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const loading = ref(false)
const enrolling = ref(false)
const home = ref<{ campaign: LotteryCampaign | null } | null>(null)
const myData = ref<LotteryMyData | null>(null)
const recentWinners = ref<LotteryPublicWinner[]>([])
const participantsOpen = ref(false)
const participantsLoading = ref(false)
const participants = ref<{ masked_email: string; entry_count: number }[]>([])
const participantsUpdatedAt = ref(new Date())
const now = ref(new Date())
let clockTimer: ReturnType<typeof setInterval> | null = null

const isSingleDraw = computed(() => home.value?.campaign?.draw_schedule_type === 'single')
const isUSDMode = computed(() => home.value?.campaign?.usage_mode === 'usd')
const usageMetricLabel = computed(() => isUSDMode.value
  ? t(isSingleDraw.value ? 'lotteryCampaign.cumulativeCost' : 'lotteryCampaign.todayCost')
  : t(isSingleDraw.value ? 'lotteryCampaign.cumulativeTokens' : 'lotteryCampaign.todayTokens'))
const drawTimeLabel = computed(() => isSingleDraw.value ? t('lotteryCampaign.drawTime') : t('lotteryCampaign.nextDraw'))
const campaignDescription = computed(() => {
  const description = home.value?.campaign?.description
  if (description) return description
  return isSingleDraw.value ? t('lotteryCampaign.defaultOneTimeDescription') : t('lotteryCampaign.defaultDescription')
})
// 与管理员预览使用同一组稳定颜色，确保两端看到的权重分布一致。
const WHEEL_COLORS = ['#0f766e', '#2563eb', '#d97706', '#be123c', '#7c3aed', '#0891b2']
const currentMaskedEmail = computed(() => maskParticipantEmail(authStore.user?.email ?? ''))
const totalParticipantWeight = computed(() => participants.value.reduce((sum, item) => sum + participantWeight(item), 0))
const wheelWinners = computed(() => recentWinners.value.filter(winner => winner.is_current_round))
const winnerEmails = computed(() => new Set(wheelWinners.value.map(winner => winner.masked_email)))
const wheelSegments = computed(() => {
  let cursor = 0
  const occurrences = new Map<string, number>()
  const totals = new Map<string, number>()
  for (const item of participants.value) totals.set(item.masked_email, (totals.get(item.masked_email) ?? 0) + 1)
  return participants.value.map((item, index) => {
    const start = cursor
    const weight = participantWeight(item)
    cursor += weight / totalParticipantWeight.value * 360
    const angle = (start + cursor) / 2
    const occurrence = (occurrences.get(item.masked_email) ?? 0) + 1
    occurrences.set(item.masked_email, occurrence)
    const displayEmail = (totals.get(item.masked_email) ?? 0) > 1 ? `${item.masked_email} · ${occurrence}` : item.masked_email
    return { ...item, displayEmail, key: `${item.masked_email}-${index}`, color: WHEEL_COLORS[index % WHEEL_COLORS.length], start, end: cursor, isCurrent: item.masked_email === currentMaskedEmail.value, isWinner: winnerEmails.value.has(item.masked_email), shortEmail: displayEmail.slice(0, 12), labelStyle: { transform: `rotate(${angle}deg) translateY(-132px) rotate(${-angle}deg)` } }
  })
})
const userWheelStyle = computed(() => ({ background: wheelSegments.value.length ? `conic-gradient(${wheelSegments.value.map(segment => `${segment.color} ${segment.start}deg ${segment.end}deg`).join(', ')})` : '#475569' }))
const rankedParticipants = computed(() => wheelSegments.value.map(segment => ({ ...segment, share: Math.round(participantWeight(segment) / totalParticipantWeight.value * 1000) / 10 })).sort((a, b) => b.entry_count - a.entry_count))
const myParticipant = computed(() => wheelSegments.value.find(segment => segment.isCurrent) ?? null)
const myProbability = computed(() => myParticipant.value ? (participantWeight(myParticipant.value) / totalParticipantWeight.value * 100).toFixed(1) : '0.0')
const drawStatus = computed(() => {
  if (myData.value?.round_completed || wheelWinners.value.length) return 'completed'
  const drawAt = nextDrawAt.value ? new Date(nextDrawAt.value).getTime() : 0
  return drawAt && drawAt <= now.value.getTime() ? 'drawing' : 'open'
})
const drawStatusClass = computed(() => ({ open: 'bg-emerald-500', drawing: 'bg-amber-500', completed: 'bg-gray-400' })[drawStatus.value])
const drawStatusBadgeClass = computed(() => ({ open: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300', drawing: 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300', completed: 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300' })[drawStatus.value])

function participantWeight(item: { entry_count: number }): number { return Math.max(1, Number(item.entry_count) || 1) }
function maskParticipantEmail(email: string): string {
  const at = email.indexOf('@')
  if (at <= 0) return email ? '****' : ''
  const local = Array.from(email.slice(0, at))
  return local.length > 4 ? `${local.slice(0, 3).join('')}****${local.slice(-2).join('')}${email.slice(at)}` : `${local[0]}****${email.slice(at)}`
}

const progressPct = computed(() => {
  const threshold = thresholdUsage.value
  if (threshold <= 0) return 0
  return Math.min(100, Math.round((currentUsage.value / threshold) * 100))
})
const currentUsage = computed(() => isUSDMode.value ? (myData.value?.today_cost_microusd ?? 0) : (myData.value?.today_tokens ?? 0))
const thresholdUsage = computed(() => isUSDMode.value ? (home.value?.campaign?.threshold_cost_microusd ?? 0) : (home.value?.campaign?.threshold_tokens ?? 0))
const remainingUsage = computed(() => Math.max(0, thresholdUsage.value - currentUsage.value))
const progressHintText = computed(() => {
  if (remainingUsage.value <= 0) return t('lotteryCampaign.thresholdReached')
  return t(isUSDMode.value ? 'lotteryCampaign.costToThreshold' : 'lotteryCampaign.tokensToThreshold', { amount: formatUsage(remainingUsage.value) })
})

const nextDrawAt = computed(() => myData.value?.next_draw_at ?? home.value?.campaign?.draw_at ?? null)
const remainingMs = computed(() => {
  if (!nextDrawAt.value) return null
  const target = new Date(nextDrawAt.value).getTime()
  if (!Number.isFinite(target)) return null
  return Math.max(0, target - now.value.getTime())
})
const isImminent = computed(() => remainingMs.value !== null && remainingMs.value > 0 && remainingMs.value <= 60 * 60 * 1000)
const nextDrawText = computed(() => {
  const raw = nextDrawAt.value
  return raw ? formatDateTime(raw) : t('lotteryCampaign.pendingDraw')
})

interface CountdownSegment {
  value: string
  label: string
}
const countdownSegments = computed<CountdownSegment[]>(() => {
  const ms = remainingMs.value
  if (ms === null || ms <= 0) return []
  const totalSeconds = Math.floor(ms / 1000)
  const days = Math.floor(totalSeconds / 86_400)
  const hours = Math.floor((totalSeconds % 86_400) / 3_600)
  const minutes = Math.floor((totalSeconds % 3_600) / 60)
  const seconds = totalSeconds % 60
  const pad = (n: number) => String(n).padStart(2, '0')
  const segments: CountdownSegment[] = []
  if (days > 0) segments.push({ value: String(days), label: t('lotteryCampaign.countdownDays') })
  segments.push({ value: pad(hours), label: t('lotteryCampaign.countdownHours') })
  segments.push({ value: pad(minutes), label: t('lotteryCampaign.countdownMinutes') })
  segments.push({ value: pad(seconds), label: t('lotteryCampaign.countdownSeconds') })
  return segments
})
const hasCountdown = computed(() => countdownSegments.value.length > 0)

const prizeTiers = computed<LotteryPrizeTier[]>(() => home.value?.campaign?.prize_tiers ?? [])
const isSinglePrize = computed(() => prizeTiers.value.length === 1)
const totalWinnerSlots = computed(() => prizeTiers.value.reduce((acc, tier) => acc + tier.winner_count, 0))
const totalPoolCents = computed(() => prizeTiers.value.reduce((acc, tier) => acc + tier.reward_amount_cents * tier.winner_count, 0))
const rulesText = computed(() => home.value?.campaign?.rules_text ?? '')

const participantCount = computed(() => myData.value?.participant_count ?? 0)

async function openParticipants() {
  if (participantsLoading.value || !home.value?.campaign) return
  participantsOpen.value = true
  participantsLoading.value = true
  try {
    const result = await lotteryCampaignsAPI.getLotteryParticipants(home.value.campaign.id)
    participants.value = result.items
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('lotteryCampaign.participantsLoadFailed')))
  } finally {
    participantsLoading.value = false
  }
}
const entryCount = computed(() => myData.value?.entry_count ?? 0)
const maxEntries = computed(() => home.value?.campaign?.max_entries_per_user ?? 1)
const showLadder = computed(() => home.value?.campaign?.entry_mode === 'stepped' && maxEntries.value > 1 && maxEntries.value <= 12)
const isWeightedLottery = computed(() => showLadder.value)
const entryMetricLabel = computed(() => t(isWeightedLottery.value ? 'lotteryCampaign.weightCount' : 'lotteryCampaign.entryCount'))
const entryLadderLabel = computed(() => t(isWeightedLottery.value ? 'lotteryCampaign.weightLadder' : 'lotteryCampaign.entryLadder'))
const ladderHintText = computed(() => {
  const campaign = home.value?.campaign
  if (!campaign) return ''
  if (entryCount.value >= maxEntries.value) return t('lotteryCampaign.ladderMaxed')
  const step = isUSDMode.value ? campaign.entry_step_cost_microusd : campaign.entry_step_tokens
  const tokens = currentUsage.value
  if (step <= 0 || entryCount.value <= 0) return t('lotteryCampaign.ladderStart')
  const nextAt = thresholdUsage.value + entryCount.value * step
  const need = Math.max(0, nextAt - tokens)
  return t('lotteryCampaign.ladderNext', { amount: formatUsage(need) })
})

const entryStatusTitle = computed(() => {
  const status = myData.value?.entry_status ?? 'not_eligible'
  return t(`lotteryCampaign.entryStatuses.${status}`)
})
const entryStatusDescription = computed(() => {
  const mode = home.value?.campaign?.participation_mode
  if (entryCount.value <= 0) return t(isUSDMode.value ? 'lotteryCampaign.needMoreCost' : 'lotteryCampaign.needMoreTokens')
  if (mode === 'manual' && myData.value?.entry_status !== 'enrolled') return t('lotteryCampaign.manualReady')
  return t('lotteryCampaign.readyForDraw')
})

interface TierAccent {
  card: string
  medal: string
  label: string
}
const TIER_ACCENTS: TierAccent[] = [
  {
    card: 'border-amber-200 bg-gradient-to-br from-amber-50 to-white dark:border-amber-500/30 dark:from-amber-900/20 dark:to-dark-800',
    medal: 'bg-gradient-to-br from-amber-400 to-amber-500 text-white shadow-sm shadow-amber-500/30',
    label: 'text-amber-700 dark:text-amber-200',
  },
  {
    card: 'border-gray-200 bg-gradient-to-br from-gray-50 to-white dark:border-dark-600 dark:from-dark-700/40 dark:to-dark-800',
    medal: 'bg-gradient-to-br from-gray-300 to-gray-400 text-white shadow-sm shadow-gray-400/30',
    label: 'text-gray-700 dark:text-dark-200',
  },
  {
    card: 'border-orange-200 bg-gradient-to-br from-orange-50 to-white dark:border-orange-500/30 dark:from-orange-900/20 dark:to-dark-800',
    medal: 'bg-gradient-to-br from-orange-400 to-orange-500 text-white shadow-sm shadow-orange-500/30',
    label: 'text-orange-700 dark:text-orange-200',
  },
]
const DEFAULT_ACCENT: TierAccent = {
  card: 'border-gray-100 dark:border-dark-700',
  medal: 'bg-gradient-to-br from-primary-400 to-primary-500 text-white shadow-sm shadow-primary-500/30',
  label: 'text-gray-700 dark:text-dark-200',
}
function tierAccent(index: number): TierAccent {
  return TIER_ACCENTS[index] ?? DEFAULT_ACCENT
}

async function load(): Promise<void> {
  loading.value = true
  try {
    home.value = await lotteryCampaignsAPI.getActiveLotteryCampaign()
    if (home.value.campaign) {
      myData.value = await lotteryCampaignsAPI.getMyLotteryCampaignData(home.value.campaign.id)
      await Promise.all([loadWinners(), loadParticipants()])
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('lotteryCampaign.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function loadWinners(): Promise<void> {
  const campaignId = home.value?.campaign?.id
  if (!campaignId) return
  try {
    const { items } = await lotteryCampaignsAPI.getRecentLotteryWinners(campaignId, 50)
    recentWinners.value = items
  } catch {
    // Recent winners are supplementary; keep the page usable if this fails.
  }
}

async function loadParticipants(): Promise<void> {
  const campaignId = home.value?.campaign?.id
  if (!campaignId) return
  try { const { items } = await lotteryCampaignsAPI.getLotteryParticipants(campaignId); participants.value = items; participantsUpdatedAt.value = new Date() } catch { /* supplementary */ }
}

async function enroll(): Promise<void> {
  if (!home.value?.campaign) return
  enrolling.value = true
  try {
    await lotteryCampaignsAPI.enrollLotteryCampaign(home.value.campaign.id)
    myData.value = await lotteryCampaignsAPI.getMyLotteryCampaignData(home.value.campaign.id)
    void loadParticipants()
    void loadWinners()
    appStore.showSuccess(t('lotteryCampaign.enrolled'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('lotteryCampaign.enrollFailed')))
  } finally {
    enrolling.value = false
  }
}

function formatTokens(value?: number): string {
  return formatTokenMillions(value ?? 0)
}

function formatUsage(value: number): string {
  if (!isUSDMode.value) return formatTokens(value)
  return new Intl.NumberFormat(locale.value, { style: 'currency', currency: 'USD', maximumFractionDigits: 4 }).format(value / 1_000_000)
}

function formatCents(cents?: number): string {
  return new Intl.NumberFormat(locale.value, { style: 'currency', currency: 'CNY' }).format((cents ?? 0) / 100)
}

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

function formatRelativeTime(value: string | Date): string {
  const target = new Date(value).getTime()
  if (!Number.isFinite(target)) return ''
  const diffSec = Math.round((target - now.value.getTime()) / 1000)
  const abs = Math.abs(diffSec)
  const rtf = new Intl.RelativeTimeFormat(locale.value, { numeric: 'auto' })
  if (abs < 60) return rtf.format(Math.round(diffSec), 'second')
  if (abs < 3600) return rtf.format(Math.round(diffSec / 60), 'minute')
  if (abs < 86_400) return rtf.format(Math.round(diffSec / 3600), 'hour')
  return rtf.format(Math.round(diffSec / 86_400), 'day')
}

onMounted(() => {
  clockTimer = setInterval(() => {
    now.value = new Date()
  }, 1000)
  void load()
})

onUnmounted(() => {
  if (clockTimer) {
    clearInterval(clockTimer)
    clockTimer = null
  }
})
</script>

<style scoped>
.user-wheel { position:relative; height:100%; width:100%; border:10px solid #fff; border-radius:9999px; box-shadow:0 4px 8px rgba(15,23,42,.16); }
.user-wheel::after { content:''; position:absolute; inset:8px; border:1px solid rgba(255,255,255,.5); border-radius:inherit; }
.user-wheel-label { position:absolute; z-index:1; top:50%; left:50%; width:5.5rem; margin:-.5rem 0 0 -2.75rem; overflow:hidden; color:#fff; font-size:.625rem; font-weight:700; line-height:1rem; text-align:center; text-overflow:ellipsis; text-shadow:0 1px 2px rgba(0,0,0,.65); white-space:nowrap; transform-origin:center; }
.user-wheel-label.is-current { color:#fff; text-decoration:underline; text-decoration-thickness:2px; text-underline-offset:2px; }
.user-wheel-label.is-winner { color:#fef3c7; font-size:.7rem; text-decoration:underline; text-decoration-color:#fbbf24; text-decoration-thickness:3px; text-underline-offset:3px; }
.user-wheel-hub { position:absolute; z-index:2; inset:50% auto auto 50%; display:flex; width:5.25rem; height:5.25rem; flex-direction:column; align-items:center; justify-content:center; border:5px solid #fff; border-radius:9999px; background:#111827; color:#fff; transform:translate(-50%,-50%); }
.user-wheel-hub strong { font-size:1.25rem; line-height:1.3; }
.user-wheel-hub small { color:#d1d5db; font-size:.625rem; }
.user-wheel-pointer { position:absolute; z-index:3; top:-.2rem; left:50%; width:0; height:0; border-right:.7rem solid transparent; border-left:.7rem solid transparent; border-top:1.55rem solid #f59e0b; filter:drop-shadow(0 2px 1px rgba(15,23,42,.25)); transform:translateX(-50%); }
.dark .user-wheel { border-color:#1f2937; }
@media (max-width: 420px) { .user-wheel-label { display:none; } }
</style>
