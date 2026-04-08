const revealTargets = document.querySelectorAll('.reveal');

const reveal = (entry) => {
  if (!entry.isIntersecting) return;
  entry.target.classList.add('show');
};

const observer = new IntersectionObserver((entries) => {
  entries.forEach(reveal);
}, {
  threshold: 0.18,
});

revealTargets.forEach((el) => observer.observe(el));
