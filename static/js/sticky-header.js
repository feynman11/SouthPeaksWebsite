// static/js/sticky-header.js

window.addEventListener('scroll', function() {
  const heroSection = document.getElementById('hero-section');
  const stickyHeader = document.getElementById('sticky-header');

  // Ensure heroSection exists on the page (it might not on very short pages)
  if (heroSection && stickyHeader) {
    const scrollThreshold = heroSection.offsetHeight / 2; // When to make header sticky

    if (window.scrollY > scrollThreshold) {
      stickyHeader.classList.add('active');
    } else {
      stickyHeader.classList.remove('active');
    }
  }
});