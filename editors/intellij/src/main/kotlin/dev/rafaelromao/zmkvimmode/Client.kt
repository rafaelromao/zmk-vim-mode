package dev.rafaelromao.zmkvimmode

import com.intellij.openapi.diagnostic.logger
import java.net.StandardProtocolFamily
import java.net.UnixDomainSocketAddress
import java.nio.ByteBuffer
import java.nio.channels.SocketChannel
import java.nio.charset.StandardCharsets
import java.nio.file.Path
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean

private val LOG = logger<Client>()

/**
 * The daemon's socket protocol: newline-delimited JSON, one connection for the
 * life of the IDE.
 *
 * Everything runs on a single background thread. Nothing here may touch the
 * EDT, and nothing here may block it: the IDE's UI freezes are famous enough
 * without a keyboard daemon contributing to them.
 */
class Client(private val pluginVersion: String) {
    private val io = Executors.newSingleThreadScheduledExecutor { r ->
        Thread(r, "zmk-vim-mode").apply { isDaemon = true }
    }
    private val stopped = AtomicBoolean(false)

    @Volatile private var channel: SocketChannel? = null
    @Volatile private var lastSent: String? = null
    private var backoffMs = 250L

    /** Mode to report, or null when this client has no opinion. */
    @Volatile private var mode: String? = null

    fun start() {
        io.execute { connect() }
    }

    fun stop() {
        if (!stopped.compareAndSet(false, true)) return
        io.execute {
            send("""{"t":"bye"}""")
            closeChannel()
        }
        io.shutdown()
        runCatching { io.awaitTermination(1, TimeUnit.SECONDS) }
    }

    /** Report a mode, or null for "no opinion". Repeats are dropped. */
    fun report(newMode: String?) {
        if (stopped.get()) return
        mode = newMode
        io.execute {
            val m = newMode ?: "none"
            if (m == lastSent) return@execute
            lastSent = m
            send("""{"t":"mode","mode":"$m"}""")
        }
    }

    private fun socketPath(): Path {
        System.getenv("ZMK_VIM_MODE_SOCKET")?.takeIf { it.isNotBlank() }?.let { return Path.of(it) }
        return Path.of(System.getProperty("user.home"), ".local", "state", "zmk-vim-mode", "daemon.sock")
    }

    private fun connect() {
        if (stopped.get() || channel != null) return
        val path = socketPath()
        try {
            val ch = SocketChannel.open(StandardProtocolFamily.UNIX)
            ch.connect(UnixDomainSocketAddress.of(path))
            ch.configureBlocking(true)
            channel = ch
            backoffMs = 250
            lastSent = null
            hello()
            LOG.info("connected to $path")
        } catch (e: Exception) {
            // The daemon may simply not be running; that is not an error worth
            // shouting about, so retry quietly.
            LOG.debug("connect to $path failed: ${e.message}")
            scheduleReconnect()
        }
    }

    private fun hello() {
        val m = mode ?: "none"
        send(
            """{"v":1,"t":"hello","client":"intellij","app":"intellij",""" +
                """"pid":${ProcessHandle.current().pid()},"mode":"$m","plugin":"$pluginVersion"}"""
        )
        lastSent = m
    }

    private fun send(line: String) {
        val ch = channel ?: return
        try {
            val buf = ByteBuffer.wrap((line + "\n").toByteArray(StandardCharsets.UTF_8))
            while (buf.hasRemaining()) ch.write(buf)
        } catch (e: Exception) {
            LOG.debug("write failed: ${e.message}")
            closeChannel()
            scheduleReconnect()
        }
    }

    private fun closeChannel() {
        channel?.let { runCatching { it.close() } }
        channel = null
        lastSent = null
    }

    private fun scheduleReconnect() {
        if (stopped.get()) return
        val delay = backoffMs
        backoffMs = (backoffMs * 2).coerceAtMost(5_000)
        runCatching { io.schedule({ connect() }, delay, TimeUnit.MILLISECONDS) }
    }
}
